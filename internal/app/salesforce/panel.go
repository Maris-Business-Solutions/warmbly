package salesforce

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

// Panel is a contact's Salesforce context as the drawer and inbox show it.
type Panel struct {
	Connections []PanelConnection `json:"connections"`
	Records     []PanelRecord     `json:"records"`
	CanSync     bool              `json:"can_sync"`
}

// PanelConnection is one connected org.
type PanelConnection struct {
	ID          uuid.UUID `json:"id"`
	Label       string    `json:"label"`
	Environment string    `json:"environment"`
	InstanceURL string    `json:"instance_url"`
}

// PanelRecord is the linked Lead or Contact in one org.
type PanelRecord struct {
	LinkID          uuid.UUID          `json:"link_id"`
	ConnectionID    uuid.UUID          `json:"connection_id"`
	ConnectionLabel string             `json:"connection_label"`
	Object          string             `json:"object"`
	ID              string             `json:"id"`
	URL             string             `json:"url"`
	Name            string             `json:"name"`
	Title           string             `json:"title,omitempty"`
	Company         string             `json:"company,omitempty"`
	Email           string             `json:"email,omitempty"`
	Phone           string             `json:"phone,omitempty"`
	Status          string             `json:"status,omitempty"`
	Owner           *PanelRef          `json:"owner,omitempty"`
	Account         *PanelAccount      `json:"account,omitempty"`
	IsConverted     bool               `json:"is_converted"`
	OptedOut        bool               `json:"opted_out"`
	LeadSource      string             `json:"lead_source,omitempty"`
	Opportunities   []PanelOpportunity `json:"opportunities"`
	Tasks           []PanelTask        `json:"tasks"`
	LinkedBy        string             `json:"linked_by"`
	LastSyncedAt    *time.Time         `json:"last_synced_at,omitempty"`
	LastPushedAt    *time.Time         `json:"last_pushed_at,omitempty"`
	Sync            PanelSync          `json:"sync"`
	Stale           bool               `json:"stale"`
	Error           string             `json:"error,omitempty"`
}

// PanelRef names a related user.
type PanelRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// PanelAccount is the Contact's account.
type PanelAccount struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

// PanelOpportunity is an opportunity on the account (or the converted Lead's).
type PanelOpportunity struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Stage     string   `json:"stage"`
	Amount    *float64 `json:"amount,omitempty"`
	CloseDate string   `json:"close_date,omitempty"`
	IsClosed  bool     `json:"is_closed"`
	IsWon     bool     `json:"is_won"`
	URL       string   `json:"url"`
}

// PanelTask is a recent activity on the record.
type PanelTask struct {
	ID          string `json:"id"`
	Subject     string `json:"subject"`
	Date        string `json:"date,omitempty"`
	Status      string `json:"status"`
	OwnerName   string `json:"owner_name,omitempty"`
	URL         string `json:"url"`
	FromWarmbly bool   `json:"from_warmbly"`
}

// PanelSync is the outbox picture for the contact.
type PanelSync struct {
	Pending   int    `json:"pending"`
	Failed    int    `json:"failed"`
	Synced    int    `json:"synced"`
	LastError string `json:"last_error,omitempty"`
}

// liveTTL is how long a live read of a record is reused before the next panel
// open asks Salesforce again.
const liveTTL = 90 * time.Second

// missTTL is how long "this address is not in Salesforce" is remembered.
const missTTL = 10 * time.Minute

type liveEntry struct {
	at    time.Time
	opps  []PanelOpportunity
	tasks []PanelTask
}

var (
	liveMu    sync.Mutex
	liveCache = map[uuid.UUID]liveEntry{}
)

// ContactPanel builds a contact's Salesforce panel, linking it on the way when
// its address matches a record nobody has linked yet.
func (s *Service) ContactPanel(ctx context.Context, orgID, contactID uuid.UUID) (*Panel, error) {
	out := &Panel{Connections: []PanelConnection{}, Records: []PanelRecord{}}
	refs, err := s.Repo.ActiveConnectionsForOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if len(refs) == 0 {
		return out, nil
	}
	cts, err := s.Repo.ContactsByIDs(ctx, orgID, []uuid.UUID{contactID})
	if err != nil {
		return nil, err
	}
	ct, ok := cts[contactID]
	if !ok {
		return nil, nil
	}
	links, err := s.Repo.LinksForContact(ctx, orgID, contactID)
	if err != nil {
		return nil, err
	}
	byConn := map[uuid.UUID]models.SalesforceRecordLink{}
	for _, l := range links {
		byConn[l.ConnectionID] = l
	}
	pending, failed, synced, lastErr, _ := s.Repo.ContactActivityCounts(ctx, orgID, contactID)
	out.CanSync = true
	for _, ref := range refs {
		c, err := s.open(ctx, orgID, ref.ID)
		if err != nil {
			continue
		}
		env := configString(c.DisplayFields, "environment")
		if env == "" {
			env = "production"
		}
		out.Connections = append(out.Connections, PanelConnection{ID: c.ID, Label: c.Label, Environment: env, InstanceURL: c.instanceURL()})
		if c.Status == models.IntegrationStatusReauthRequired {
			if l, ok := byConn[c.ID]; ok {
				rec := s.recordFromLink(c, l)
				rec.Stale = true
				rec.Error = "Reconnect Salesforce to see live data"
				out.Records = append(out.Records, rec)
			}
			continue
		}
		l, linked := byConn[c.ID]
		if !linked {
			l, linked = s.linkOnView(ctx, c, ct.Email, contactID)
		}
		if !linked {
			s.settle(ctx, c)
			continue
		}
		rec := s.livePanelRecord(ctx, c, l)
		rec.Sync = PanelSync{Pending: pending, Failed: failed, Synced: synced, LastError: lastErr}
		out.Records = append(out.Records, rec)
		s.settle(ctx, c)
	}
	return out, nil
}

// linkOnView matches an unlinked contact by address the first time its panel
// opens, remembering a miss for a while so browsing costs no API calls.
func (s *Service) linkOnView(ctx context.Context, c *conn, email string, contactID uuid.UUID) (models.SalesforceRecordLink, bool) {
	key := c.ID.String() + "|" + strings.ToLower(email)
	s.missMu.Lock()
	at, missed := s.misses[key]
	s.missMu.Unlock()
	if missed && time.Since(at) < missTTL {
		return models.SalesforceRecordLink{}, false
	}
	found, err := s.matchEmails(ctx, c, []string{email})
	if err != nil {
		return models.SalesforceRecordLink{}, false
	}
	m, ok := found[strings.ToLower(email)]
	if !ok {
		s.missMu.Lock()
		s.misses[key] = time.Now()
		if len(s.misses) > 50000 {
			s.misses = map[string]time.Time{}
		}
		s.missMu.Unlock()
		return models.SalesforceRecordLink{}, false
	}
	l := linkFrom(c, contactID, m.object, m.rec, "match")
	if err := s.Repo.UpsertLink(ctx, l); err != nil {
		return models.SalesforceRecordLink{}, false
	}
	if l.OptedOut {
		s.optOutFromSalesforce(ctx, c, repository.SalesforceContact{ID: contactID, Email: email})
	}
	return *l, true
}

// recordFromLink renders what the link last saw.
func (s *Service) recordFromLink(c *conn, l models.SalesforceRecordLink) PanelRecord {
	snap := snapshotOf(l)
	rec := PanelRecord{
		LinkID: l.ID, ConnectionID: c.ID, ConnectionLabel: c.Label,
		Object: l.SObject, ID: l.RecordID, URL: c.recordURL(l.RecordID),
		Name: snap.String("Name"), Title: snap.String("Title"), Email: snap.String("Email"), Phone: snap.String("Phone"),
		Status: l.LeadStatus, IsConverted: l.IsConverted, OptedOut: l.OptedOut, LeadSource: snap.String("LeadSource"),
		LinkedBy: l.LinkedBy, LastSyncedAt: l.LastPulledAt, LastPushedAt: l.LastPushedAt,
		Opportunities: []PanelOpportunity{}, Tasks: []PanelTask{},
	}
	if rec.Name == "" {
		rec.Name = strings.TrimSpace(snap.String("FirstName") + " " + snap.String("LastName"))
	}
	if l.OwnerID != "" || l.OwnerName != "" {
		rec.Owner = &PanelRef{ID: l.OwnerID, Name: l.OwnerName}
	}
	if l.SObject == ObjectLead {
		rec.Company = l.AccountName
	} else {
		rec.Company = l.AccountName
		if l.AccountID != "" {
			rec.Account = &PanelAccount{ID: l.AccountID, Name: l.AccountName, URL: c.recordURL(l.AccountID)}
		}
	}
	if l.LastError != nil {
		rec.Error = *l.LastError
	}
	return rec
}

// livePanelRecord refreshes the record from Salesforce when the link is stale,
// and adds opportunities and recent activity.
func (s *Service) livePanelRecord(ctx context.Context, c *conn, l models.SalesforceRecordLink) PanelRecord {
	if l.LastPulledAt == nil || time.Since(*l.LastPulledAt) > liveTTL {
		rows, err := sfQuery(ctx, c, l.SObject, "Id = "+Quote(l.RecordID), 1)
		switch {
		case err != nil:
			rec := s.recordFromLink(c, l)
			rec.Stale = true
			rec.Error = describeErr(err)
			return rec
		case len(rows) == 0:
			rec := s.recordFromLink(c, l)
			rec.Stale = true
			rec.Error = "This record was deleted or is no longer visible to the connected Salesforce user"
			return rec
		default:
			next := linkFrom(c, l.ContactID, l.SObject, rows[0], l.LinkedBy)
			if next.SObject == ObjectLead && next.IsConverted && s.followConversion(ctx, c, l, rows[0]) {
				// Shown as the Contact it became; that link was just read, so
				// this goes one level deep at most.
				if links, err := s.Repo.LinksForContact(ctx, c.OrganizationID, l.ContactID); err == nil {
					for _, nl := range links {
						if nl.ConnectionID == c.ID && nl.SObject == ObjectContact {
							return s.livePanelRecord(ctx, c, nl)
						}
					}
				}
			}
			next.LastPushedAt = l.LastPushedAt
			if err := s.Repo.UpsertLink(ctx, next); err == nil {
				now := time.Now().UTC()
				next.LastPulledAt = &now
				l = *next
			}
		}
	}
	rec := s.recordFromLink(c, l)
	opps, tasks := s.related(ctx, c, l)
	rec.Opportunities, rec.Tasks = opps, tasks
	return rec
}

// related reads opportunities and recent Tasks, reused for liveTTL.
func (s *Service) related(ctx context.Context, c *conn, l models.SalesforceRecordLink) ([]PanelOpportunity, []PanelTask) {
	liveMu.Lock()
	e, ok := liveCache[l.ID]
	liveMu.Unlock()
	if ok && time.Since(e.at) < liveTTL {
		return e.opps, e.tasks
	}
	opps := []PanelOpportunity{}
	where := ""
	switch {
	case l.SObject == ObjectContact && l.AccountID != "":
		where = "AccountId = " + Quote(l.AccountID)
	case l.SObject == ObjectLead:
		if id := NormalizeID(snapshotOf(l).String("ConvertedOpportunityId")); id != "" {
			where = "Id = " + Quote(id)
		}
	}
	if where != "" {
		rows, err := c.client.QueryAll(ctx, "SELECT Id, Name, StageName, Amount, CloseDate, IsClosed, IsWon FROM Opportunity WHERE "+
			where+" ORDER BY IsClosed ASC, CloseDate DESC LIMIT 5", 5)
		if err == nil {
			for _, r := range rows {
				o := PanelOpportunity{
					ID: NormalizeID(r.String("Id")), Name: r.String("Name"), Stage: r.String("StageName"),
					CloseDate: r.String("CloseDate"), IsClosed: r.Bool("IsClosed"), IsWon: r.Bool("IsWon"),
				}
				o.URL = c.recordURL(o.ID)
				if v := r.String("Amount"); v != "" {
					if f, err := strconv.ParseFloat(v, 64); err == nil {
						o.Amount = &f
					}
				}
				opps = append(opps, o)
			}
		}
	}
	tasks := []PanelTask{}
	rows, err := c.client.QueryAll(ctx, "SELECT Id, Subject, ActivityDate, Status, Owner.Name, Description FROM Task WHERE WhoId = "+
		Quote(l.RecordID)+" ORDER BY CreatedDate DESC LIMIT 8", 8)
	if err == nil {
		for _, r := range rows {
			id := NormalizeID(r.String("Id"))
			tasks = append(tasks, PanelTask{
				ID: id, Subject: r.String("Subject"), Date: r.String("ActivityDate"), Status: r.String("Status"),
				OwnerName: r.String("Owner.Name"), URL: c.recordURL(id),
				FromWarmbly: strings.HasSuffix(strings.TrimSpace(r.String("Description")), "Logged by Warmbly"),
			})
		}
	}
	liveMu.Lock()
	liveCache[l.ID] = liveEntry{at: time.Now(), opps: opps, tasks: tasks}
	if len(liveCache) > 20000 {
		liveCache = map[uuid.UUID]liveEntry{}
	}
	liveMu.Unlock()
	return opps, tasks
}

// ForgetLive drops cached live reads and remembered misses for a contact, so
// the panel after an explicit sync shows what Salesforce holds now.
func (s *Service) ForgetLive(ctx context.Context, orgID, contactID uuid.UUID) {
	if links, err := s.Repo.LinksForContact(ctx, orgID, contactID); err == nil {
		liveMu.Lock()
		for _, l := range links {
			delete(liveCache, l.ID)
		}
		liveMu.Unlock()
	}
	cts, err := s.Repo.ContactsByIDs(ctx, orgID, []uuid.UUID{contactID})
	if err != nil {
		return
	}
	if ct, ok := cts[contactID]; ok {
		suffix := "|" + strings.ToLower(ct.Email)
		s.missMu.Lock()
		for k := range s.misses {
			if strings.HasSuffix(k, suffix) {
				delete(s.misses, k)
			}
		}
		s.missMu.Unlock()
	}
}
