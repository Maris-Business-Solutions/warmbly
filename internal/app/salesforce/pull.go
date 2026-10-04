package salesforce

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

// pullPageCap bounds how many changed records one connection reads per pass;
// the cursor carries the rest to the next pass.
const pullPageCap = 6000

// Pull reads what changed in every connected org since its cursor and applies
// it to linked contacts.
func (s *Service) Pull(ctx context.Context) error {
	refs, err := s.Repo.ActiveConnections(ctx)
	if err != nil {
		return err
	}
	for _, ref := range refs {
		if !ParseSettings(ref.ConfigCapabilities).Enabled {
			continue
		}
		c, err := s.open(ctx, ref.OrganizationID, ref.ID)
		if err != nil {
			continue
		}
		if c.Status == models.IntegrationStatusReauthRequired || s.overBudget(ctx, c) {
			continue
		}
		s.pullConnection(ctx, c)
		s.settle(ctx, c)
	}
	return nil
}

func (s *Service) pullConnection(ctx context.Context, c *conn) {
	if err := s.Repo.EnsureSyncState(ctx, c.ID, c.OrganizationID, time.Now().UTC()); err != nil {
		return
	}
	state, err := s.Repo.GetSyncState(ctx, c.ID)
	if err != nil || state == nil {
		return
	}
	var errs []string
	leadCursor, err := s.pullObject(ctx, c, ObjectLead, state.LeadCursor)
	if err != nil {
		errs = append(errs, "Leads: "+describeErr(err))
	}
	contactCursor, err := s.pullObject(ctx, c, ObjectContact, state.ContactCursor)
	if err != nil {
		errs = append(errs, "Contacts: "+describeErr(err))
	}
	oppCursor := state.OppCursor
	if c.settings.Inbound.PauseOnOpenOpportunity {
		next, err := s.pullOpportunities(ctx, c, state.OppCursor)
		if err != nil {
			errs = append(errs, "Opportunities: "+describeErr(err))
		} else {
			oppCursor = next
		}
	} else {
		// Off means only opportunities opened after it is turned on count.
		now := time.Now().UTC()
		oppCursor = &now
	}
	_ = s.Repo.SetCursors(ctx, c.ID, leadCursor, contactCursor, oppCursor, strings.Join(errs, "; "))
}

// soqlTime renders a SOQL datetime literal, which carries whole seconds.
func soqlTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05Z")
}

// advance moves a cursor to the latest timestamp read. SOQL compares whole
// seconds, so a cursor that would land in the second it started from steps a
// full second past it; otherwise a second holding more changes than one pass
// reads would be re-read forever.
func advance(cursor time.Time, latest *time.Time) *time.Time {
	start := cursor.UTC().Truncate(time.Second)
	if latest == nil {
		return &cursor
	}
	if !latest.UTC().Truncate(time.Second).After(start) {
		next := start.Add(time.Second)
		return &next
	}
	l := latest.UTC()
	return &l
}

// pullObject reads records modified at or after the cursor. Reading from the
// cursor inclusive re-reads the boundary record, which is harmless, rather
// than skipping one that shares its timestamp.
func (s *Service) pullObject(ctx context.Context, c *conn, object string, cursor *time.Time) (*time.Time, error) {
	if cursor == nil {
		now := time.Now().UTC()
		return &now, nil
	}
	where := "SystemModstamp >= " + soqlTime(*cursor) + " ORDER BY SystemModstamp ASC"
	fields := selectFields(c.settings, object)
	soql := "SELECT " + strings.Join(fields, ", ") + " FROM " + object + " WHERE " + where
	var latest *time.Time
	var applyErr error
	_, err := c.client.Query(ctx, soql, pullPageCap, func(rows []Record) bool {
		if err := s.applyPulled(ctx, c, object, rows); err != nil {
			applyErr = err
			return false
		}
		for _, r := range rows {
			if t := r.Time("SystemModstamp"); t != nil && (latest == nil || t.After(*latest)) {
				latest = t
			}
		}
		return true
	})
	if err != nil && IsCode(err, "INVALID_FIELD") {
		// A rule names a field the org dropped; keep pulling on the base set.
		soql = "SELECT " + strings.Join(baseFields[object], ", ") + " FROM " + object + " WHERE " + where
		_, err = c.client.Query(ctx, soql, pullPageCap, func(rows []Record) bool {
			_ = s.applyPulled(ctx, c, object, rows)
			for _, r := range rows {
				if t := r.Time("SystemModstamp"); t != nil && (latest == nil || t.After(*latest)) {
					latest = t
				}
			}
			return true
		})
	}
	if err == nil {
		err = applyErr
	}
	if err != nil || latest == nil {
		return cursor, err
	}
	return advance(*cursor, latest), nil
}

// applyPulled updates the links behind changed records and acts on what
// changed.
func (s *Service) applyPulled(ctx context.Context, c *conn, object string, rows []Record) error {
	if len(rows) == 0 {
		return nil
	}
	ids := make([]string, 0, len(rows))
	byID := map[string]Record{}
	for _, r := range rows {
		id := NormalizeID(r.String("Id"))
		ids = append(ids, id)
		byID[id] = r
	}
	links, err := s.Repo.LinksForRecords(ctx, c.ID, ids)
	if err != nil || len(links) == 0 {
		return err
	}
	var contactIDs []uuid.UUID
	for _, l := range links {
		contactIDs = append(contactIDs, l.ContactID)
	}
	contacts, err := s.Repo.ContactsByIDs(ctx, c.OrganizationID, contactIDs)
	if err != nil {
		return err
	}
	for _, old := range links {
		if old.SObject != object {
			continue
		}
		rec := byID[old.RecordID]
		ct, ok := contacts[old.ContactID]
		if rec == nil || !ok {
			continue
		}
		next := linkFrom(c, old.ContactID, object, rec, old.LinkedBy)

		if object == ObjectLead && next.IsConverted && !old.IsConverted {
			if !s.followConversion(ctx, c, old, rec) {
				// The Contact it became is not readable; keep the converted Lead.
				_ = s.Repo.UpsertLink(ctx, next)
			}
			if c.settings.Inbound.PauseOnConverted {
				s.hold(ctx, c, ct, "Salesforce: the Lead was converted")
			}
			continue
		}
		if object == ObjectLead && next.LeadStatus != old.LeadStatus && containsFold(c.settings.Inbound.PauseOnStatuses, next.LeadStatus) {
			s.hold(ctx, c, ct, "Salesforce: Lead Status is "+next.LeadStatus)
		}
		if next.OptedOut && !old.OptedOut {
			s.optOutFromSalesforce(ctx, c, ct)
		}
		s.pullFields(ctx, c, object, rec, ct)
		if err := s.Repo.UpsertLink(ctx, next); err != nil {
			log.Warn().Err(err).Msg("salesforce: could not refresh link")
		}
	}
	return nil
}

// followConversion moves a converted Lead's link to the Contact it became,
// and reports whether it could.
func (s *Service) followConversion(ctx context.Context, c *conn, old models.SalesforceRecordLink, lead Record) bool {
	contactID := NormalizeID(lead.String("ConvertedContactId"))
	if contactID == "" {
		return false
	}
	rows, err := sfQuery(ctx, c, ObjectContact, "Id = "+Quote(contactID), 1)
	if err != nil || len(rows) == 0 {
		return false
	}
	l := linkFrom(c, old.ContactID, ObjectContact, rows[0], old.LinkedBy)
	return s.Repo.UpsertLink(ctx, l) == nil
}

// hold parks the contact's outreach everywhere, as a Salesforce rule.
func (s *Service) hold(ctx context.Context, c *conn, ct repository.SalesforceContact, reason string) {
	if s.Holds == nil {
		return
	}
	if _, err := s.Holds.HoldLeadEverywhere(ctx, ct.ID, nil, reason, models.LeadHoldSourceCRM); err != nil {
		log.Warn().Err(err).Str("contact", ct.ID.String()).Msg("salesforce: could not hold lead")
	}
}

// optOutFromSalesforce honours Email Opt Out set in Salesforce.
func (s *Service) optOutFromSalesforce(ctx context.Context, c *conn, ct repository.SalesforceContact) {
	mode := c.settings.Inbound.OptOut
	if mode != "both" && mode != "from_salesforce" {
		return
	}
	if s.Suppression != nil {
		_ = s.Suppression.UpsertSuppressedRecipient(ctx, &models.SuppressedRecipient{
			OrganizationID: c.OrganizationID,
			Email:          strings.ToLower(ct.Email),
			Kind:           models.SuppressionKindEmail,
			Reason:         "Email Opt Out is set in Salesforce",
			Source:         models.DeliverabilityEventUnsubscribe,
			Metadata:       map[string]any{"via": "salesforce", "connection_id": c.ID.String()},
		})
	}
	if s.Subscription != nil {
		_ = s.Subscription.SetSubscribedByEmail(ctx, c.OrganizationID, ct.Email, false)
	}
}

// pullFields writes the pull rules' values onto the Warmbly contact.
func (s *Service) pullFields(ctx context.Context, c *conn, object string, rec Record, ct repository.SalesforceContact) {
	rules := c.settings.rulesFor(object, DirectionPull)
	if len(rules) == 0 || s.Contacts == nil {
		return
	}
	upd := &models.UpdateContact{}
	custom := map[string]string{}
	for k, v := range ct.CustomFields {
		custom[k] = v
	}
	changed, customChanged := false, false
	set := func(dst **string, cur, v string, policy string) {
		if v == "" || cur == v || (policy == PolicyIfEmpty && cur != "") {
			return
		}
		val := v
		*dst = &val
		changed = true
	}
	for _, r := range rules {
		if isEngagementField(r.Warmbly) {
			continue
		}
		v := strings.TrimSpace(rec.String(r.Salesforce))
		switch r.Warmbly {
		case "first_name":
			set(&upd.FirstName, ct.FirstName, v, r.Policy)
		case "last_name":
			set(&upd.LastName, ct.LastName, v, r.Policy)
		case "company":
			set(&upd.Company, ct.Company, v, r.Policy)
		case "phone":
			set(&upd.Phone, ct.Phone, v, r.Policy)
		default:
			key, ok := strings.CutPrefix(r.Warmbly, "custom:")
			if !ok || v == "" {
				continue
			}
			cur := custom[key]
			if cur == v || (r.Policy == PolicyIfEmpty && cur != "") {
				continue
			}
			custom[key] = v
			customChanged = true
		}
	}
	if customChanged {
		upd.CustomFields = &custom
		changed = true
	}
	if !changed {
		return
	}
	actor := uuid.Nil
	if c.ConnectedByUserID != nil {
		actor = *c.ConnectedByUserID
	}
	if _, xerr := s.Contacts.Update(ctx, actor.String(), ct.ID.String(), c.OrganizationID, upd); xerr != nil {
		log.Warn().Str("contact", ct.ID.String()).Str("error", xerr.Message).Msg("salesforce: could not apply pulled fields")
	}
}

// pullOpportunities holds Contacts whose account gained an open opportunity
// since the cursor, and returns where it read up to. Each opportunity is read
// once, so a member who resumes a lead is not overruled by the next pass.
func (s *Service) pullOpportunities(ctx context.Context, c *conn, cursor *time.Time) (*time.Time, error) {
	if cursor == nil {
		now := time.Now().UTC()
		return &now, nil
	}
	rows, err := c.client.QueryAll(ctx, "SELECT AccountId, CreatedDate FROM Opportunity WHERE IsClosed = false AND AccountId != null AND CreatedDate >= "+
		soqlTime(*cursor)+" ORDER BY CreatedDate ASC LIMIT 2000", 2000)
	if err != nil {
		return cursor, err
	}
	var accounts []string
	var latest *time.Time
	seen := map[string]bool{}
	for _, r := range rows {
		if t := r.Time("CreatedDate"); t != nil && (latest == nil || t.After(*latest)) {
			latest = t
		}
		id := NormalizeID(r.String("AccountId"))
		if id != "" && !seen[id] {
			seen[id] = true
			accounts = append(accounts, id)
		}
	}
	next := advance(*cursor, latest)
	links, err := s.Repo.LinksForAccounts(ctx, c.ID, accounts)
	if err != nil {
		return cursor, err
	}
	if len(links) == 0 {
		return next, nil
	}
	ids := make([]uuid.UUID, 0, len(links))
	for _, l := range links {
		ids = append(ids, l.ContactID)
	}
	contacts, err := s.Repo.ContactsByIDs(ctx, c.OrganizationID, ids)
	if err != nil {
		return cursor, err
	}
	for _, l := range links {
		if ct, ok := contacts[l.ContactID]; ok {
			s.hold(ctx, c, ct, fmt.Sprintf("Salesforce: an open opportunity exists on %s", orDefault(l.AccountName, "the account")))
		}
	}
	return next, nil
}

func containsFold(list []string, v string) bool {
	if v == "" {
		return false
	}
	for _, x := range list {
		if strings.EqualFold(strings.TrimSpace(x), v) {
			return true
		}
	}
	return false
}

// Prune drops processed outbox rows past the retention window.
func (s *Service) Prune(ctx context.Context) error {
	_, err := s.Repo.PruneActivities(ctx, time.Now().UTC().AddDate(0, 0, -activityRetentionDays))
	return err
}

// activityRetentionDays is how long the activity log keeps processed rows.
const activityRetentionDays = 30
