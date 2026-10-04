package salesforce

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/app/integration"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

// baseFields are read for every record, whatever the field rules name.
var baseFields = map[string][]string{
	ObjectLead: {"Id", "Email", "Name", "FirstName", "LastName", "Title", "Phone", "Company", "Status",
		"OwnerId", "Owner.Name", "IsConverted", "ConvertedContactId", "ConvertedOpportunityId", "HasOptedOutOfEmail",
		"LeadSource", "SystemModstamp"},
	ObjectContact: {"Id", "Email", "Name", "FirstName", "LastName", "Title", "Phone", "AccountId", "Account.Name",
		"OwnerId", "Owner.Name", "HasOptedOutOfEmail", "LeadSource", "SystemModstamp"},
}

// selectFields is the SELECT list for an object: the base set plus every field
// a rule reads or compares against.
func selectFields(st Settings, object string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(f string) {
		k := strings.ToLower(f)
		if !seen[k] {
			seen[k] = true
			out = append(out, f)
		}
	}
	for _, f := range baseFields[object] {
		add(f)
	}
	for _, r := range st.FieldMap {
		if r.Object == object {
			add(r.Salesforce)
		}
	}
	return out
}

// sfQuery runs a SELECT and, if a field rule names a field the org does not
// have, retries with the base fields so one bad rule never stops the sync.
func sfQuery(ctx context.Context, c *conn, object, where string, limit int) ([]Record, error) {
	fields := selectFields(c.settings, object)
	q := func(fs []string) ([]Record, error) {
		soql := "SELECT " + strings.Join(fs, ", ") + " FROM " + object + " WHERE " + where
		if limit > 0 {
			soql += fmt.Sprintf(" LIMIT %d", limit)
		}
		return c.client.QueryAll(ctx, soql, 0)
	}
	rows, err := q(fields)
	if err != nil && IsCode(err, "INVALID_FIELD") && len(fields) > len(baseFields[object]) {
		return q(baseFields[object])
	}
	return rows, err
}

// matched is a Salesforce record an address resolved to.
type matched struct {
	object string
	rec    Record
}

// matchEmails finds the Lead or Contact behind each address. A converted Lead
// is never a match (its Contact is), and when an address is both, the
// connection's preference decides.
func (s *Service) matchEmails(ctx context.Context, c *conn, emails []string) (map[string]matched, error) {
	out := map[string]matched{}
	uniq := dedupeLower(emails)
	for start := 0; start < len(uniq); start += 100 {
		chunk := uniq[start:min(start+100, len(uniq))]
		in := QuoteList(chunk)
		contacts, err := sfQuery(ctx, c, ObjectContact, "Email IN "+in+" ORDER BY LastModifiedDate DESC", 0)
		if err != nil {
			return nil, err
		}
		leads, err := sfQuery(ctx, c, ObjectLead, "IsConverted = false AND Email IN "+in+" ORDER BY LastModifiedDate DESC", 0)
		if err != nil {
			return nil, err
		}
		first, second := contacts, leads
		firstObj, secondObj := ObjectContact, ObjectLead
		if c.settings.Matching.Prefer == "lead" {
			first, second, firstObj, secondObj = leads, contacts, ObjectLead, ObjectContact
		}
		for _, r := range first {
			e := strings.ToLower(r.String("Email"))
			if _, ok := out[e]; !ok && e != "" {
				out[e] = matched{object: firstObj, rec: r}
			}
		}
		for _, r := range second {
			e := strings.ToLower(r.String("Email"))
			if _, ok := out[e]; !ok && e != "" {
				out[e] = matched{object: secondObj, rec: r}
			}
		}
	}
	return out, nil
}

// linkFrom builds a link from a fetched record.
func linkFrom(c *conn, contactID uuid.UUID, object string, r Record, by string) *models.SalesforceRecordLink {
	snap := map[string]any{}
	for k, v := range r {
		if k == "attributes" {
			continue
		}
		snap[k] = v
	}
	raw, _ := json.Marshal(snap)
	l := &models.SalesforceRecordLink{
		OrganizationID:   c.OrganizationID,
		ConnectionID:     c.ID,
		ContactID:        contactID,
		SObject:          object,
		RecordID:         NormalizeID(r.String("Id")),
		OwnerID:          NormalizeID(r.String("OwnerId")),
		OwnerName:        r.String("Owner.Name"),
		OptedOut:         r.Bool("HasOptedOutOfEmail"),
		Snapshot:         raw,
		LinkedBy:         by,
		RecordModifiedAt: r.Time("SystemModstamp"),
	}
	if object == ObjectLead {
		l.LeadStatus = r.String("Status")
		l.IsConverted = r.Bool("IsConverted")
		l.AccountName = r.String("Company")
	} else {
		l.AccountID = NormalizeID(r.String("AccountId"))
		l.AccountName = r.String("Account.Name")
	}
	return l
}

func snapshotOf(l models.SalesforceRecordLink) Record {
	out := Record{}
	_ = json.Unmarshal(l.Snapshot, &out)
	return out
}

// --- creation ---------------------------------------------------------------

// createRequest is one person to create in Salesforce.
type createRequest struct {
	contact   repository.SalesforceContact
	object    string
	ownerID   string
	overrides map[string]any
}

// creationHeaders apply the connection's assignment-rule choice and let a
// duplicate rule that only alerts still save.
func creationHeaders(st Settings) []Header {
	assign := "FALSE"
	if st.Matching.RunAssignmentRules {
		assign = "TRUE"
	}
	return []Header{{Key: "Sforce-Auto-Assign", Value: assign}, {Key: "Sforce-Duplicate-Rule-Header", Value: "allowSave=true"}}
}

// createRecords inserts people as Leads or Contacts. Required fields fall back
// to what the address says, because Salesforce refuses a Lead without a last
// name or a company.
func (s *Service) createRecords(ctx context.Context, c *conn, reqs []createRequest) (map[uuid.UUID]matched, map[uuid.UUID]error) {
	ok := map[uuid.UUID]matched{}
	failed := map[uuid.UUID]error{}
	byObject := map[string][]createRequest{}
	for _, r := range reqs {
		byObject[r.object] = append(byObject[r.object], r)
	}
	meta, _ := s.describe(ctx, c)
	for object, list := range byObject {
		records := make([]map[string]any, 0, len(list))
		for _, r := range list {
			records = append(records, allowedPicklists(meta, object, s.newRecordFields(c, object, r)))
		}
		res, err := c.client.Create(ctx, object, records, creationHeaders(c.settings)...)
		if err != nil {
			for _, r := range list {
				failed[r.contact.ID] = err
			}
			continue
		}
		var ids []string
		idx := map[string]uuid.UUID{}
		for i, sr := range res {
			if i >= len(list) {
				break
			}
			if e := sr.Err(); e != nil {
				failed[list[i].contact.ID] = e
				continue
			}
			ids = append(ids, sr.ID)
			idx[sr.ID] = list[i].contact.ID
		}
		if len(ids) == 0 {
			continue
		}
		rows, err := sfQuery(ctx, c, object, "Id IN "+QuoteList(ids), 0)
		if err != nil {
			// Created but unreadable: link with the id alone; the pull fills it in.
			for id, cid := range idx {
				ok[cid] = matched{object: object, rec: Record{"Id": id}}
			}
			continue
		}
		for _, r := range rows {
			if cid, found := idx[NormalizeID(r.String("Id"))]; found {
				ok[cid] = matched{object: object, rec: r}
			}
		}
	}
	return ok, failed
}

func (s *Service) newRecordFields(c *conn, object string, r createRequest) map[string]any {
	ct := r.contact
	f := map[string]any{"Email": ct.Email}
	local, domain, _ := strings.Cut(ct.Email, "@")
	if ct.FirstName != "" {
		f["FirstName"] = ct.FirstName
	}
	last := ct.LastName
	if last == "" {
		last = local
	}
	f["LastName"] = truncate(last, 80)
	if ct.Phone != "" {
		f["Phone"] = ct.Phone
	}
	if object == ObjectLead {
		company := ct.Company
		if company == "" {
			company = domain
		}
		if company == "" {
			company = "[not provided]"
		}
		f["Company"] = truncate(company, 255)
		if st := c.settings.Matching.LeadStatus; st != "" {
			f["Status"] = st
		}
	}
	if src := c.settings.Matching.LeadSource; src != "" {
		f["LeadSource"] = src
	}
	if t := ct.CustomFields["title"]; t != "" {
		f["Title"] = truncate(t, 128)
	}
	for _, rule := range c.settings.rulesFor(object, DirectionPush) {
		if strings.Contains(rule.Salesforce, ".") || isEngagementField(rule.Warmbly) {
			continue
		}
		if v := warmblyValue(ct, nil, rule.Warmbly); v != "" {
			f[rule.Salesforce] = v
		}
	}
	for k, v := range safeOverrides(r.overrides) {
		f[k] = v
	}
	if r.ownerID != "" && !c.settings.Matching.RunAssignmentRules {
		f["OwnerId"] = r.ownerID
	}
	return f
}

// allowedPicklists drops a Lead Source or Status the org's restricted
// picklist would refuse, so one unknown value never blocks every creation.
func allowedPicklists(meta *cachedMeta, object string, f map[string]any) map[string]any {
	if meta == nil {
		return f
	}
	d := meta.contact
	if object == ObjectLead {
		d = meta.lead
	}
	for _, name := range []string{"LeadSource", "Status"} {
		v, ok := f[name].(string)
		if !ok {
			continue
		}
		fd := d.Field(name)
		if fd == nil {
			delete(f, name)
			continue
		}
		if !fd.Restricted {
			continue
		}
		allowed := false
		for _, p := range fd.PicklistValues {
			if p.Active && strings.EqualFold(p.Value, v) {
				f[name] = p.Value
				allowed = true
				break
			}
		}
		if !allowed {
			delete(f, name)
		}
	}
	return f
}

// createOwner picks the owner of a created record.
func (s *Service) createOwner(ctx context.Context, c *conn, senderEmail string) string {
	switch c.settings.Matching.Owner {
	case "fixed":
		return c.settings.Matching.OwnerID
	case "sender":
		if id := s.userByEmail(ctx, c, senderEmail); id != "" {
			return id
		}
	}
	return c.sfUserID()
}

// userByEmail finds the active Salesforce user with an address, cached with
// the connection's metadata.
func (s *Service) userByEmail(ctx context.Context, c *conn, email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return ""
	}
	m, err := s.describe(ctx, c)
	if err != nil {
		return ""
	}
	s.metaMu.Lock()
	id, ok := m.userByEmail[email]
	s.metaMu.Unlock()
	if ok {
		return id
	}
	rows, err := c.client.QueryAll(ctx, "SELECT Id FROM User WHERE IsActive = true AND Email = "+Quote(email)+" LIMIT 1", 1)
	if err != nil {
		return ""
	}
	if len(rows) > 0 {
		id = NormalizeID(rows[0].String("Id"))
	}
	s.metaMu.Lock()
	m.userByEmail[email] = id
	s.metaMu.Unlock()
	return id
}

// --- field values -----------------------------------------------------------

// warmblyValue reads a Warmbly-side field for a contact.
func warmblyValue(ct repository.SalesforceContact, eng *repository.SalesforceEngagement, field string) string {
	switch field {
	case "first_name":
		return ct.FirstName
	case "last_name":
		return ct.LastName
	case "company":
		return ct.Company
	case "phone":
		return ct.Phone
	case "email":
		return ct.Email
	}
	if key, ok := strings.CutPrefix(field, "custom:"); ok {
		return strings.TrimSpace(ct.CustomFields[key])
	}
	if eng == nil {
		return ""
	}
	switch field {
	case "engagement.last_campaign":
		return eng.LastCampaign
	case "engagement.last_sent_at":
		return sfDateTime(eng.LastSentAt)
	case "engagement.last_reply_at":
		return sfDateTime(eng.LastReplyAt)
	case "engagement.reply_intent":
		return eng.ReplyIntent
	case "engagement.status":
		return eng.Status
	}
	return ""
}

func sfDateTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format("2006-01-02T15:04:05Z")
}

// pushChanges computes the field updates a contact's rules call for, against
// what Salesforce last showed.
func pushChanges(st Settings, l models.SalesforceRecordLink, ct repository.SalesforceContact, eng *repository.SalesforceEngagement) map[string]any {
	snap := snapshotOf(l)
	out := map[string]any{}
	for _, rule := range st.rulesFor(l.SObject, DirectionPush) {
		if strings.Contains(rule.Salesforce, ".") {
			continue
		}
		v := warmblyValue(ct, eng, rule.Warmbly)
		if v == "" {
			continue
		}
		_, known := snap[rule.Salesforce]
		cur := snap.String(rule.Salesforce)
		// A field never read is unknown, and "only fill blanks" never writes blind.
		if rule.Policy == PolicyIfEmpty && (!known || cur != "") {
			continue
		}
		if cur == v {
			continue
		}
		out[rule.Salesforce] = v
	}
	return out
}

// --- on-demand sync ---------------------------------------------------------

// ensureLinks returns each contact's link, matching unlinked ones and, when
// createAs is set, creating the people still missing. senderEmail feeds the
// owner rule.
func (s *Service) ensureLinks(ctx context.Context, c *conn, contacts []repository.SalesforceContact, createAs, senderEmail string, overrides map[string]any) (map[uuid.UUID]models.SalesforceRecordLink, map[uuid.UUID]error) {
	ids := make([]uuid.UUID, 0, len(contacts))
	for _, ct := range contacts {
		ids = append(ids, ct.ID)
	}
	failed := map[uuid.UUID]error{}
	links, err := s.Repo.LinksForContacts(ctx, c.ID, ids)
	if err != nil {
		for _, id := range ids {
			failed[id] = err
		}
		return nil, failed
	}
	var missing []repository.SalesforceContact
	for _, ct := range contacts {
		l, ok := links[ct.ID]
		if !ok {
			missing = append(missing, ct)
			continue
		}
		// A converted Lead's link moves to the Contact it became.
		if l.SObject == ObjectLead && l.IsConverted {
			missing = append(missing, ct)
			delete(links, ct.ID)
		}
	}
	if len(missing) == 0 {
		return links, failed
	}
	emails := make([]string, 0, len(missing))
	for _, ct := range missing {
		emails = append(emails, ct.Email)
	}
	found, err := s.matchEmails(ctx, c, emails)
	if err != nil {
		for _, ct := range missing {
			failed[ct.ID] = err
		}
		return links, failed
	}
	var toCreate []createRequest
	for _, ct := range missing {
		if m, ok := found[strings.ToLower(ct.Email)]; ok {
			l := linkFrom(c, ct.ID, m.object, m.rec, "match")
			if err := s.Repo.UpsertLink(ctx, l); err != nil {
				failed[ct.ID] = err
				continue
			}
			links[ct.ID] = *l
			if l.OptedOut {
				s.optOutFromSalesforce(ctx, c, ct)
			}
			continue
		}
		if createAs == "" {
			continue
		}
		obj := ObjectLead
		if createAs == "contact" {
			obj = ObjectContact
		}
		toCreate = append(toCreate, createRequest{contact: ct, object: obj, ownerID: s.createOwner(ctx, c, senderEmail), overrides: overrides})
	}
	if len(toCreate) > 0 {
		created, cerr := s.createRecords(ctx, c, toCreate)
		for id, e := range cerr {
			failed[id] = e
		}
		for id, m := range created {
			l := linkFrom(c, id, m.object, m.rec, "created")
			if err := s.Repo.UpsertLink(ctx, l); err != nil {
				failed[id] = err
				continue
			}
			links[id] = *l
		}
	}
	return links, failed
}

// applyUpdates writes field changes to linked records, one batch per object,
// and refreshes each link's snapshot with what was written.
func (s *Service) applyUpdates(ctx context.Context, c *conn, changes map[uuid.UUID]map[string]any, links map[uuid.UUID]models.SalesforceRecordLink) map[uuid.UUID]error {
	failed := map[uuid.UUID]error{}
	byObject := map[string][]uuid.UUID{}
	for cid, ch := range changes {
		if len(ch) == 0 {
			continue
		}
		l := links[cid]
		if l.SObject == ObjectLead && l.IsConverted {
			continue
		}
		byObject[l.SObject] = append(byObject[l.SObject], cid)
	}
	for object, cids := range byObject {
		records := make([]map[string]any, 0, len(cids))
		for _, cid := range cids {
			rec := map[string]any{"Id": links[cid].RecordID}
			for k, v := range changes[cid] {
				rec[k] = v
			}
			records = append(records, rec)
		}
		res, err := c.client.Update(ctx, object, records)
		if err != nil {
			for _, cid := range cids {
				failed[cid] = err
			}
			continue
		}
		for i, sr := range res {
			if i >= len(cids) {
				break
			}
			cid := cids[i]
			if e := sr.Err(); e != nil {
				failed[cid] = e
				_ = s.Repo.SetLinkError(ctx, links[cid].ID, e.Error())
				continue
			}
			l := links[cid]
			snap := snapshotOf(l)
			for k, v := range changes[cid] {
				snap[k] = v
			}
			if v, ok := changes[cid]["Status"].(string); ok {
				l.LeadStatus = v
			}
			if v, ok := changes[cid]["HasOptedOutOfEmail"].(bool); ok {
				l.OptedOut = v
			}
			l.Snapshot, _ = json.Marshal(snap)
			_ = s.Repo.UpsertLink(ctx, &l)
			links[cid] = l
		}
	}
	return failed
}

// SyncContact matches (or, with createAs, creates) one contact's record in
// every active Salesforce connection of the organization, or only connID, and
// pushes the mapped fields.
func (s *Service) SyncContact(ctx context.Context, orgID, contactID uuid.UUID, connID *uuid.UUID, createAs string) error {
	if createAs != "" && createAs != "lead" && createAs != "contact" {
		return errx.New(errx.BadRequest, "create_as must be lead or contact")
	}
	cts, err := s.Repo.ContactsByIDs(ctx, orgID, []uuid.UUID{contactID})
	if err != nil {
		return err
	}
	ct, ok := cts[contactID]
	if !ok {
		return errx.New(errx.NotFound, "contact not found")
	}
	refs, err := s.Repo.ActiveConnectionsForOrg(ctx, orgID)
	if err != nil {
		return err
	}
	ran := false
	for _, ref := range refs {
		if connID != nil && ref.ID != *connID {
			continue
		}
		ran = true
		c, err := s.open(ctx, orgID, ref.ID)
		if err != nil {
			return err
		}
		err = s.syncOne(ctx, c, ct, createAs, nil)
		s.settle(ctx, c)
		if err != nil {
			return sfError(err)
		}
	}
	if !ran {
		return errx.New(errx.NotFound, "no connected Salesforce org")
	}
	return nil
}

func (s *Service) syncOne(ctx context.Context, c *conn, ct repository.SalesforceContact, createAs string, overrides map[string]any) error {
	links, failed := s.ensureLinks(ctx, c, []repository.SalesforceContact{ct}, createAs, "", overrides)
	if err := failed[ct.ID]; err != nil {
		return err
	}
	l, ok := links[ct.ID]
	if !ok {
		return nil
	}
	eng, _ := s.Repo.Engagement(ctx, c.OrganizationID, []uuid.UUID{ct.ID})
	var e *repository.SalesforceEngagement
	if v, ok := eng[ct.ID]; ok {
		e = &v
	}
	ch := pushChanges(c.settings, l, ct, e)
	snap := snapshotOf(l)
	for k, v := range safeOverrides(overrides) {
		if snap.String(k) != fmt.Sprint(v) {
			ch[k] = v
		}
	}
	if len(ch) == 0 {
		return s.Repo.MarkLinksPushed(ctx, []uuid.UUID{l.ID})
	}
	if errs := s.applyUpdates(ctx, c, map[uuid.UUID]map[string]any{ct.ID: ch}, links); errs[ct.ID] != nil {
		return errs[ct.ID]
	}
	return s.Repo.MarkLinksPushed(ctx, []uuid.UUID{l.ID})
}

// PushContacts is the contextual "push to Salesforce" action: every contact is
// matched or created (as the connection's create_as) and its fields pushed.
func (s *Service) PushContacts(ctx context.Context, orgID, connID uuid.UUID, contacts []integration.PushContact) (*integration.PushResult, error) {
	c, err := s.open(ctx, orgID, connID)
	if err != nil {
		return nil, err
	}
	defer s.settle(ctx, c)
	ids := make([]uuid.UUID, 0, len(contacts))
	for _, p := range contacts {
		ids = append(ids, p.ID)
	}
	byID, err := s.Repo.ContactsByIDs(ctx, orgID, ids)
	if err != nil {
		return nil, err
	}
	var list []repository.SalesforceContact
	for _, id := range ids {
		if ct, ok := byID[id]; ok && strings.TrimSpace(ct.Email) != "" {
			list = append(list, ct)
		}
	}
	links, failed := s.ensureLinks(ctx, c, list, c.settings.Matching.CreateAs, "", nil)
	eng, _ := s.Repo.Engagement(ctx, orgID, ids)
	changes := map[uuid.UUID]map[string]any{}
	for _, ct := range list {
		l, ok := links[ct.ID]
		if !ok {
			continue
		}
		var e *repository.SalesforceEngagement
		if v, ok := eng[ct.ID]; ok {
			e = &v
		}
		changes[ct.ID] = pushChanges(c.settings, l, ct, e)
	}
	for id, e := range s.applyUpdates(ctx, c, changes, links) {
		failed[id] = e
	}
	var pushed []uuid.UUID
	res := &integration.PushResult{Provider: string(models.IntegrationSalesforce)}
	for _, p := range contacts {
		rr := integration.PushRecordResult{ContactID: p.ID, Email: p.Email, OK: true}
		_, known := byID[p.ID]
		switch {
		case !known || strings.TrimSpace(p.Email) == "":
			rr.OK, rr.Error = false, "contact has no email"
		case failed[p.ID] != nil:
			rr.OK, rr.Error = false, truncate(failed[p.ID].Error(), 240)
		default:
			l, ok := links[p.ID]
			if !ok {
				rr.OK, rr.Error = false, "not created: creating records is off for this connection"
			} else {
				pushed = append(pushed, l.ID)
			}
		}
		if rr.OK {
			res.Pushed++
		} else {
			res.Failed++
		}
		res.Results = append(res.Results, rr)
	}
	_ = s.Repo.MarkLinksPushed(ctx, pushed)
	return res, nil
}

// UpsertFromEvent is the automation action "create or update Salesforce
// record": the event's contact is matched, or created as the connection's
// create_as, then updated.
func (s *Service) UpsertFromEvent(ctx context.Context, orgID, connID uuid.UUID, data map[string]any) error {
	err := s.upsertFromEvent(ctx, orgID, connID, data)
	if errors.Is(err, ErrSessionExpired) {
		return fmt.Errorf("%w: %v", integration.ErrPushReauth, err)
	}
	return err
}

func (s *Service) upsertFromEvent(ctx context.Context, orgID, connID uuid.UUID, data map[string]any) error {
	c, err := s.open(ctx, orgID, connID)
	if err != nil {
		return err
	}
	defer s.settle(ctx, c)
	var ct repository.SalesforceContact
	if id, perr := uuid.Parse(str(data, "contact_id")); perr == nil {
		if m, err := s.Repo.ContactsByIDs(ctx, orgID, []uuid.UUID{id}); err == nil {
			ct = m[id]
		}
	}
	if ct.ID == uuid.Nil {
		email := strings.ToLower(str(data, "contact_email", "invitee_email", "email", "recipient"))
		if email == "" {
			return nil
		}
		m, err := s.Repo.ContactsByEmails(ctx, orgID, []string{email})
		if err != nil {
			return err
		}
		ct = m[email]
	}
	overrides, _ := data["_salesforce_fields"].(map[string]any)
	if ct.ID == uuid.Nil {
		return s.upsertBare(ctx, c, data, overrides)
	}
	return s.syncOne(ctx, c, ct, c.settings.Matching.CreateAs, overrides)
}

// upsertBare finds or creates someone who is not a Warmbly contact (a meeting
// invitee, a form or webhook lead) from the event's own fields. Nothing is
// linked, because there is no contact to link.
func (s *Service) upsertBare(ctx context.Context, c *conn, data map[string]any, overrides map[string]any) error {
	email := strings.ToLower(str(data, "contact_email", "invitee_email", "email", "recipient"))
	if !strings.Contains(email, "@") {
		return nil
	}
	ct := repository.SalesforceContact{
		ID:           uuid.New(),
		Email:        email,
		FirstName:    str(data, "first_name", "contact_first_name"),
		LastName:     str(data, "last_name", "contact_last_name"),
		Company:      str(data, "company", "contact_company"),
		Phone:        str(data, "phone", "contact_phone"),
		CustomFields: map[string]string{},
	}
	if ct.FirstName == "" && ct.LastName == "" {
		if full := str(data, "contact_name", "invitee_name", "name"); full != "" {
			first, last, _ := strings.Cut(full, " ")
			ct.FirstName, ct.LastName = first, strings.TrimSpace(last)
		}
	}
	found, err := s.matchEmails(ctx, c, []string{email})
	if err != nil {
		return err
	}
	if m, ok := found[email]; ok {
		ch := map[string]any{}
		for k, v := range safeOverrides(overrides) {
			if m.rec.String(k) != fmt.Sprint(v) {
				ch[k] = v
			}
		}
		if len(ch) == 0 || (m.object == ObjectLead && m.rec.Bool("IsConverted")) {
			return nil
		}
		ch["Id"] = NormalizeID(m.rec.String("Id"))
		res, err := c.client.Update(ctx, m.object, []map[string]any{ch})
		if err != nil {
			return err
		}
		if len(res) > 0 {
			return res[0].Err()
		}
		return nil
	}
	obj := ObjectLead
	if c.settings.Matching.CreateAs == "contact" {
		obj = ObjectContact
	}
	_, failed := s.createRecords(ctx, c, []createRequest{{contact: ct, object: obj, ownerID: s.createOwner(ctx, c, ""), overrides: overrides}})
	return failed[ct.ID]
}

// Unlink drops a contact's link to a record; nothing changes in Salesforce.
func (s *Service) Unlink(ctx context.Context, orgID, contactID, linkID uuid.UUID) error {
	links, err := s.Repo.LinksForContact(ctx, orgID, contactID)
	if err != nil {
		return err
	}
	for _, l := range links {
		if l.ID == linkID {
			_, err := s.Repo.DeleteLink(ctx, orgID, linkID)
			return err
		}
	}
	return errx.New(errx.NotFound, "link not found")
}

// safeOverrides keeps an automation's own field values that name a plain field
// of the record: never its Id, which would retarget the write, or a related one.
func safeOverrides(in map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range in {
		if strings.EqualFold(k, "Id") || strings.Contains(k, ".") || !validSalesforceField(k) {
			continue
		}
		out[k] = v
	}
	return out
}

func dedupeLower(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.ToLower(strings.TrimSpace(v))
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
