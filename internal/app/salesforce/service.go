package salesforce

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/warmbly/warmbly/internal/app/cipher"
	"github.com/warmbly/warmbly/internal/app/contact"
	"github.com/warmbly/warmbly/internal/app/integration"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

// LeadHolder parks a contact's outreach in every campaign they are a lead of.
type LeadHolder interface {
	HoldLeadEverywhere(ctx context.Context, contactID uuid.UUID, until *time.Time, reason, source string) ([]uuid.UUID, error)
}

// Suppressor adds an address to the organization's suppression list.
type Suppressor interface {
	UpsertSuppressedRecipient(ctx context.Context, entry *models.SuppressedRecipient) error
}

// SubscriptionWriter flips a contact's subscribed flag by address.
type SubscriptionWriter interface {
	SetSubscribedByEmail(ctx context.Context, orgID uuid.UUID, email string, subscribed bool) error
}

// Deps is everything the sync talks to.
type Deps struct {
	Repo         repository.SalesforceRepository
	Integrations integration.Service
	Cipher       cipher.CipherService
	Contacts     contact.ContactService
	Holds        LeadHolder
	Suppression  Suppressor
	Subscription SubscriptionWriter
}

// Service is the native Salesforce sync.
type Service struct {
	Deps
	recorder *Recorder

	metaMu sync.Mutex
	meta   map[uuid.UUID]*cachedMeta

	missMu sync.Mutex
	misses map[string]time.Time

	// syncing holds the connections a "Sync now" is running for.
	syncing sync.Map
}

// NewService builds the sync.
func NewService(d Deps) *Service {
	return &Service{
		Deps:     d,
		recorder: NewRecorder(d.Repo, d.Cipher),
		meta:     map[uuid.UUID]*cachedMeta{},
		misses:   map[string]time.Time{},
	}
}

// Recorder is the event sink half, for wiring into the webhook fan-out.
func (s *Service) Recorder() *Recorder { return s.recorder }

// --- connection access ------------------------------------------------------

// conn is one Salesforce connection resolved for work.
type conn struct {
	*models.IntegrationConnection
	settings Settings
	client   *Client
	usage    Usage
	usageMu  sync.Mutex
}

func (c *conn) instanceURL() string {
	v, _ := integration.SalesforceInstanceURL(configString(c.DisplayFields, "instance_url"))
	return v
}
func (c *conn) sfUserID() string { return NormalizeID(configString(c.DisplayFields, "sf_user_id")) }

// recordURL is a record's address in the org; Salesforce redirects /<id> to
// the right Lightning page for any object.
func (c *conn) recordURL(id string) string {
	if id == "" || c.instanceURL() == "" {
		return ""
	}
	return strings.TrimRight(c.instanceURL(), "/") + "/" + id
}

type tokenSource struct {
	svc         integration.Service
	orgID, conn uuid.UUID
	mu          sync.Mutex
	token, inst string
}

func (t *tokenSource) Token(ctx context.Context, force bool) (string, string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.token != "" && !force {
		return t.token, t.inst, nil
	}
	acc, err := t.svc.ProviderAccess(ctx, t.orgID, t.conn, force)
	if err != nil {
		return "", "", err
	}
	t.token, t.inst = acc.Token, acc.InstanceURL
	return t.token, t.inst, nil
}

// open resolves an org-owned Salesforce connection and a client for it.
func (s *Service) open(ctx context.Context, orgID, connID uuid.UUID) (*conn, error) {
	ic, err := s.Integrations.GetConnection(ctx, orgID, connID)
	if err != nil {
		return nil, err
	}
	if ic == nil || ic.Provider != models.IntegrationSalesforce {
		return nil, errx.New(errx.NotFound, "Salesforce connection not found")
	}
	c := &conn{IntegrationConnection: ic, settings: ParseSettings(ic.ConfigCapabilities)}
	if !hasSettings(ic.ConfigCapabilities) {
		if rows, err := s.Integrations.ListFieldMappings(ctx, orgID, connID); err == nil {
			c.settings.withLegacyMappings(rows)
		}
	}
	c.client = NewClient(&tokenSource{svc: s.Integrations, orgID: orgID, conn: connID}, func(u Usage) {
		c.usageMu.Lock()
		c.usage = u
		c.usageMu.Unlock()
	})
	return c, nil
}

// settle records the calls a unit of work made and the org's API reading.
func (s *Service) settle(ctx context.Context, c *conn) {
	calls := c.client.Calls()
	c.usageMu.Lock()
	u := c.usage
	c.usageMu.Unlock()
	if calls == 0 && u.Max == 0 {
		return
	}
	_ = s.Repo.EnsureSyncState(ctx, c.ID, c.OrganizationID, time.Now().UTC())
	_ = s.Repo.RecordUsage(ctx, c.ID, u.Used, u.Max, calls)
}

// budget is the connection's daily call allowance.
func budget(st Settings, state *models.SalesforceSyncState) int {
	if st.DailyAPIBudget > 0 {
		return st.DailyAPIBudget
	}
	if state != nil && state.APIMax > 0 {
		return max(state.APIMax/5, 1000)
	}
	return 5000
}

// overBudget reports whether background work should wait for tomorrow: either
// Warmbly spent its share, or the org as a whole is nearly out of calls.
func (s *Service) overBudget(ctx context.Context, c *conn) bool {
	state, err := s.Repo.GetSyncState(ctx, c.ID)
	if err != nil || state == nil {
		return false
	}
	if state.CallsToday >= budget(c.settings, state) {
		return true
	}
	return state.APIMax > 0 && state.APISeenAt != nil && time.Since(*state.APISeenAt) < time.Hour &&
		float64(state.APIUsed) >= 0.95*float64(state.APIMax)
}

// Connected starts a new connection's pull cursor at the moment it connected,
// so the first pull reads changes from then on.
// A brand-new connection is saved with the defaults, sync on; reconnecting an
// existing one keeps whatever it had.
func (s *Service) Connected(ctx context.Context, ic *models.IntegrationConnection) {
	_ = s.Repo.EnsureSyncState(ctx, ic.ID, ic.OrganizationID, time.Now().UTC())
	if !hasSettings(ic.ConfigCapabilities) && time.Since(ic.CreatedAt) < 5*time.Minute {
		if _, err := s.SaveSettings(ctx, ic.OrganizationID, ic.ID, DefaultSettings()); err != nil {
			log.Warn().Err(err).Str("connection", ic.ID.String()).Msg("salesforce: could not save default settings")
		}
	}
	s.recorder.forget(ic.OrganizationID)
}

func hasSettings(raw json.RawMessage) bool {
	var wrap map[string]json.RawMessage
	if json.Unmarshal(raw, &wrap) != nil {
		return false
	}
	_, ok := wrap[settingsKey]
	return ok
}

// --- settings ---------------------------------------------------------------

// GetSettings returns a connection's settings.
func (s *Service) GetSettings(ctx context.Context, orgID, connID uuid.UUID) (Settings, error) {
	c, err := s.open(ctx, orgID, connID)
	if err != nil {
		return Settings{}, err
	}
	return c.settings, nil
}

// SaveSettings validates and stores a connection's settings.
func (s *Service) SaveSettings(ctx context.Context, orgID, connID uuid.UUID, next Settings) (Settings, error) {
	if next.Writeback.LeadStatusOnReply == nil {
		next.Writeback.LeadStatusOnReply = map[string]string{}
	}
	if next.Inbound.PauseOnStatuses == nil {
		next.Inbound.PauseOnStatuses = []string{}
	}
	if next.FieldMap == nil {
		next.FieldMap = []FieldRule{}
	}
	for i := range next.FieldMap {
		next.FieldMap[i].Salesforce = strings.TrimSpace(next.FieldMap[i].Salesforce)
		next.FieldMap[i].Warmbly = strings.TrimSpace(next.FieldMap[i].Warmbly)
	}
	next.Matching.OwnerID = NormalizeID(next.Matching.OwnerID)
	if err := next.Validate(); err != nil {
		return Settings{}, errx.NewWithIdentifier(errx.BadRequest, "invalid_salesforce_settings", err.Error())
	}
	c, err := s.open(ctx, orgID, connID)
	if err != nil {
		return Settings{}, err
	}
	if err := s.Integrations.SetConfigKey(ctx, orgID, c.ID, settingsKey, next); err != nil {
		return Settings{}, err
	}
	s.recorder.forget(orgID)
	if next.Enabled {
		_ = s.Repo.EnsureSyncState(ctx, connID, orgID, time.Now().UTC())
	}
	return next, nil
}

// --- overview ---------------------------------------------------------------

// Check is one permission or setup probe.
type Check struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

// Overview is the connection's health page.
type Overview struct {
	ConnectionID    uuid.UUID                       `json:"connection_id"`
	Label           string                          `json:"label"`
	Status          string                          `json:"status"`
	Health          string                          `json:"health"`
	HealthDetail    string                          `json:"health_detail,omitempty"`
	Org             OverviewOrg                     `json:"org"`
	API             OverviewAPI                     `json:"api"`
	Counts          models.SalesforceActivityCounts `json:"counts"`
	LastPullAt      *time.Time                      `json:"last_pull_at,omitempty"`
	LastPullError   string                          `json:"last_pull_error,omitempty"`
	SettingsEnabled bool                            `json:"settings_enabled"`
	Checks          []Check                         `json:"checks,omitempty"`
}

// OverviewOrg names the connected org.
type OverviewOrg struct {
	ID          string `json:"id,omitempty"`
	InstanceURL string `json:"instance_url"`
	Environment string `json:"environment"`
	LoginHost   string `json:"login_host"`
	UserID      string `json:"user_id,omitempty"`
	Account     string `json:"account,omitempty"`
}

// OverviewAPI is the API budget picture.
type OverviewAPI struct {
	Used       int `json:"used"`
	Max        int `json:"max"`
	CallsToday int `json:"calls_today"`
	Budget     int `json:"budget"`
}

// Overview builds the health page; withChecks also probes permissions.
func (s *Service) Overview(ctx context.Context, orgID, connID uuid.UUID, withChecks bool) (*Overview, error) {
	c, err := s.open(ctx, orgID, connID)
	if err != nil {
		return nil, err
	}
	env := configString(c.DisplayFields, "environment")
	if env == "" {
		env = "production"
	}
	host := configString(c.DisplayFields, "login_host")
	if host == "" {
		host = "login.salesforce.com"
	}
	out := &Overview{
		ConnectionID: c.ID,
		Label:        c.Label,
		Status:       string(c.Status),
		Health:       c.Health,
		Org: OverviewOrg{
			ID:          configString(c.DisplayFields, "sf_org_id"),
			InstanceURL: c.instanceURL(),
			Environment: env,
			LoginHost:   host,
			UserID:      c.sfUserID(),
			Account:     c.ExternalAccountName,
		},
		SettingsEnabled: c.settings.Enabled,
	}
	if c.HealthDetail != nil {
		out.HealthDetail = *c.HealthDetail
	}
	if counts, err := s.Repo.ActivityCounts(ctx, c.ID); err == nil {
		out.Counts = counts
	}
	state, _ := s.Repo.GetSyncState(ctx, c.ID)
	if withChecks {
		if u, err := c.client.Limits(ctx); err == nil {
			c.usageMu.Lock()
			c.usage = u
			c.usageMu.Unlock()
		}
		out.Checks = s.checks(ctx, c)
		s.settle(ctx, c)
		state, _ = s.Repo.GetSyncState(ctx, c.ID)
	}
	if state != nil {
		out.API = OverviewAPI{Used: state.APIUsed, Max: state.APIMax, CallsToday: state.CallsToday}
		out.LastPullAt = state.LastPullAt
		out.LastPullError = state.LastPullError
	}
	out.API.Budget = budget(c.settings, state)
	return out, nil
}

// checks probes what the connected user can do, as plain answers an admin can
// act on.
func (s *Service) checks(ctx context.Context, c *conn) []Check {
	var out []Check
	add := func(key, label string, ok bool, detail string) {
		out = append(out, Check{Key: key, Label: label, OK: ok, Detail: detail})
	}
	for _, obj := range []string{ObjectLead, ObjectContact, "Task"} {
		d, err := c.client.Describe(ctx, obj)
		if err != nil {
			add(strings.ToLower(obj)+"_access", obj+" access", false, err.Error())
			continue
		}
		switch obj {
		case "Task":
			ok := d.Createable
			detail := ""
			if !ok {
				detail = "The connected user cannot create Tasks, so activity cannot be logged."
			} else if f := d.Field("TaskSubtype"); f == nil || !f.Createable {
				detail = "Tasks are logged without the email icon: TaskSubtype is not writable for this user."
			}
			add("task_create", "Log activity as Tasks", ok, detail)
		default:
			detail := ""
			if !d.Createable {
				detail = "The connected user cannot create " + obj + "s."
			}
			add(strings.ToLower(obj)+"_create", "Create "+obj+"s", d.Createable, detail)
			upd := d.Updateable
			detail = ""
			if !upd {
				detail = "The connected user cannot edit " + obj + "s, so status and opt-out writeback is off."
			} else if f := d.Field("HasOptedOutOfEmail"); f == nil || !f.Updateable {
				detail = "Email Opt Out is not editable for this user, so unsubscribes cannot be written back."
			}
			add(strings.ToLower(obj)+"_update", "Update "+obj+"s", upd, detail)
		}
	}
	if _, err := c.client.ListViews(ctx, ObjectLead); err != nil {
		add("list_views", "Read list views", false, err.Error())
	} else {
		add("list_views", "Read list views", true, "")
	}
	return out
}

// --- metadata ---------------------------------------------------------------

type cachedMeta struct {
	at          time.Time
	lead        *Describe
	contact     *Describe
	closedTask  string
	userByEmail map[string]string
}

const metaTTL = 15 * time.Minute

// describe returns the Lead and Contact describes, cached per connection.
func (s *Service) describe(ctx context.Context, c *conn) (*cachedMeta, error) {
	s.metaMu.Lock()
	m := s.meta[c.ID]
	s.metaMu.Unlock()
	if m != nil && time.Since(m.at) < metaTTL {
		return m, nil
	}
	lead, err := c.client.Describe(ctx, ObjectLead)
	if err != nil {
		return nil, err
	}
	ct, err := c.client.Describe(ctx, ObjectContact)
	if err != nil {
		return nil, err
	}
	m = &cachedMeta{at: time.Now(), lead: lead, contact: ct, closedTask: "Completed", userByEmail: map[string]string{}}
	rows, err := c.client.QueryAll(ctx, "SELECT ApiName, MasterLabel FROM TaskStatus WHERE IsClosed = true ORDER BY SortOrder LIMIT 1", 1)
	if err != nil {
		rows, err = c.client.QueryAll(ctx, "SELECT MasterLabel FROM TaskStatus WHERE IsClosed = true ORDER BY SortOrder LIMIT 1", 1)
	}
	if err == nil && len(rows) > 0 {
		if v := rows[0].String("ApiName"); v != "" {
			m.closedTask = v
		} else if v := rows[0].String("MasterLabel"); v != "" {
			m.closedTask = v
		}
	}
	s.metaMu.Lock()
	s.meta[c.ID] = m
	s.metaMu.Unlock()
	return m, nil
}

// FieldInfo is one field as the mapping editor shows it.
type FieldInfo struct {
	Name       string       `json:"name"`
	Label      string       `json:"label"`
	Type       string       `json:"type"`
	Createable bool         `json:"createable"`
	Updateable bool         `json:"updateable"`
	Calculated bool         `json:"calculated"`
	Custom     bool         `json:"custom"`
	Picklist   []PickOption `json:"picklist,omitempty"`
}

// PickOption is a picklist value.
type PickOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// Metadata is what the settings screens pick from.
type Metadata struct {
	LeadFields    []FieldInfo  `json:"lead_fields"`
	ContactFields []FieldInfo  `json:"contact_fields"`
	LeadStatuses  []PickOption `json:"lead_statuses"`
	LeadSources   []PickOption `json:"lead_sources"`
}

// Metadata returns the field and picklist catalogue of a connection.
func (s *Service) Metadata(ctx context.Context, orgID, connID uuid.UUID) (*Metadata, error) {
	c, err := s.open(ctx, orgID, connID)
	if err != nil {
		return nil, err
	}
	defer s.settle(ctx, c)
	m, err := s.describe(ctx, c)
	if err != nil {
		return nil, sfError(err)
	}
	out := &Metadata{
		LeadFields:    fieldInfos(m.lead, ObjectLead),
		ContactFields: fieldInfos(m.contact, ObjectContact),
		LeadStatuses:  picklist(m.lead, "Status"),
		LeadSources:   picklist(m.lead, "LeadSource"),
	}
	return out, nil
}

// relatedReadFields are lookups worth reading through, offered as pull-only.
var relatedReadFields = map[string][]FieldInfo{
	ObjectContact: {
		{Name: "Account.Name", Label: "Account Name", Type: "string"},
		{Name: "Account.Website", Label: "Account Website", Type: "url"},
		{Name: "Account.Industry", Label: "Account Industry", Type: "picklist"},
	},
	ObjectLead: {},
}

func fieldInfos(d *Describe, object string) []FieldInfo {
	out := make([]FieldInfo, 0, len(d.Fields))
	for _, f := range d.Fields {
		switch f.Type {
		case "address", "location", "base64", "anyType", "complexvalue":
			continue
		}
		fi := FieldInfo{
			Name: f.Name, Label: f.Label, Type: f.Type,
			Createable: f.Createable, Updateable: f.Updateable,
			Calculated: f.Calculated || f.AutoNumber, Custom: f.Custom,
		}
		if f.Type == "picklist" || f.Type == "multipicklist" {
			for _, p := range f.PicklistValues {
				if p.Active {
					fi.Picklist = append(fi.Picklist, PickOption{Value: p.Value, Label: p.Label})
				}
			}
		}
		out = append(out, fi)
	}
	out = append(out, relatedReadFields[object]...)
	sort.SliceStable(out, func(i, j int) bool { return strings.ToLower(out[i].Label) < strings.ToLower(out[j].Label) })
	return out
}

func picklist(d *Describe, field string) []PickOption {
	f := d.Field(field)
	if f == nil {
		return []PickOption{}
	}
	out := make([]PickOption, 0, len(f.PicklistValues))
	for _, p := range f.PicklistValues {
		if p.Active {
			out = append(out, PickOption{Value: p.Value, Label: p.Label})
		}
	}
	return out
}

// statusRank is a Lead status's position in the picklist, -1 when unknown.
func statusRank(d *Describe, status string) int {
	if d == nil || status == "" {
		return -1
	}
	f := d.Field("Status")
	if f == nil {
		return -1
	}
	n := 0
	for _, p := range f.PicklistValues {
		if !p.Active {
			continue
		}
		if strings.EqualFold(p.Value, status) {
			return n
		}
		n++
	}
	return -1
}

// User is a Salesforce user for owner pickers.
type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Users searches the org's active users.
func (s *Service) Users(ctx context.Context, orgID, connID uuid.UUID, q string) ([]User, error) {
	c, err := s.open(ctx, orgID, connID)
	if err != nil {
		return nil, err
	}
	defer s.settle(ctx, c)
	soql := "SELECT Id, Name, Email FROM User WHERE IsActive = true AND UserType = 'Standard'"
	if q = strings.TrimSpace(q); q != "" {
		like := likeQuote(q)
		soql += " AND (Name LIKE " + like + " OR Email LIKE " + like + ")"
	}
	soql += " ORDER BY Name LIMIT 50"
	rows, err := c.client.QueryAll(ctx, soql, 50)
	if err != nil {
		return nil, sfError(err)
	}
	out := make([]User, 0, len(rows))
	for _, r := range rows {
		out = append(out, User{ID: NormalizeID(r.String("Id")), Name: r.String("Name"), Email: r.String("Email")})
	}
	return out, nil
}

// ListViewInfo is a list view for the import picker.
type ListViewInfo struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Object string `json:"object"`
}

// ListViews lists an object's list views.
func (s *Service) ListViews(ctx context.Context, orgID, connID uuid.UUID, object string) ([]ListViewInfo, error) {
	if object != ObjectLead && object != ObjectContact {
		return nil, errx.New(errx.BadRequest, "object must be Lead or Contact")
	}
	c, err := s.open(ctx, orgID, connID)
	if err != nil {
		return nil, err
	}
	defer s.settle(ctx, c)
	views, err := c.client.ListViews(ctx, object)
	if err != nil {
		return nil, sfError(err)
	}
	out := make([]ListViewInfo, 0, len(views))
	for _, v := range views {
		out = append(out, ListViewInfo{ID: v.ID, Label: v.Label, Object: object})
	}
	return out, nil
}

// CampaignInfo is a Salesforce Campaign for the import picker.
type CampaignInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	Type        string `json:"type"`
	MemberCount int    `json:"member_count"`
}

// Campaigns searches the org's Salesforce Campaigns.
func (s *Service) Campaigns(ctx context.Context, orgID, connID uuid.UUID, q string) ([]CampaignInfo, error) {
	c, err := s.open(ctx, orgID, connID)
	if err != nil {
		return nil, err
	}
	defer s.settle(ctx, c)
	soql := "SELECT Id, Name, Status, Type, NumberOfLeads, NumberOfContacts FROM Campaign WHERE IsDeleted = false"
	if q = strings.TrimSpace(q); q != "" {
		soql += " AND Name LIKE " + likeQuote(q)
	}
	soql += " ORDER BY IsActive DESC, LastModifiedDate DESC LIMIT 50"
	rows, err := c.client.QueryAll(ctx, soql, 50)
	if err != nil {
		return nil, sfError(err)
	}
	out := make([]CampaignInfo, 0, len(rows))
	for _, r := range rows {
		out = append(out, CampaignInfo{
			ID: NormalizeID(r.String("Id")), Name: r.String("Name"), Status: r.String("Status"), Type: r.String("Type"),
			MemberCount: atoi(r.String("NumberOfLeads")) + atoi(r.String("NumberOfContacts")),
		})
	}
	return out, nil
}

// --- helpers ----------------------------------------------------------------

// sfError turns a Salesforce failure into an answer the dashboard can show.
// Anything that is not Salesforce's own refusal answers with a fixed sentence;
// the cause is logged.
func sfError(err error) error {
	if err == nil {
		return nil
	}
	var xe *errx.Error
	if errors.As(err, &xe) {
		return xe
	}
	switch {
	case errors.Is(err, integration.ErrPushReauth), errors.Is(err, ErrSessionExpired):
		return errx.NewWithIdentifier(errx.Conflict, "salesforce_reconnect_required", "Salesforce needs to be reconnected before this can run.")
	case errors.Is(err, ErrRateLimited):
		return errx.NewWithIdentifier(errx.TooManyRequests, "salesforce_rate_limited", "Your Salesforce org has used its API requests for today.")
	}
	var ae *APIError
	if errors.As(err, &ae) {
		return errx.NewWithIdentifier(errx.BadRequest, "salesforce_error", "Salesforce: "+ae.Error())
	}
	var ue *url.Error
	if errors.As(err, &ue) {
		return errx.NewWithIdentifier(errx.BadRequest, "salesforce_unreachable", "Could not reach Salesforce. Try again shortly.")
	}
	log.Error().Err(err).Msg("salesforce request failed")
	return errx.InternalError()
}

// describeErr is the text an activity row or link keeps about a failure: what
// Salesforce said, or a fixed sentence, never an internal error or a URL that
// carries a contact's address.
func describeErr(err error) string {
	var ae *APIError
	switch {
	case errors.As(err, &ae):
		return ae.Error()
	case errors.Is(err, integration.ErrPushReauth), errors.Is(err, ErrSessionExpired):
		return "Waiting for Salesforce to be reconnected"
	case errors.Is(err, ErrRateLimited):
		return "Salesforce API limit reached for today"
	}
	var ue *url.Error
	if errors.As(err, &ue) {
		return "Salesforce did not respond; this will be retried"
	}
	log.Warn().Err(err).Msg("salesforce: sync step failed")
	return "Something went wrong syncing with Salesforce; this will be retried"
}

func configString(raw json.RawMessage, key string) string {
	if len(raw) == 0 {
		return ""
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// likeQuote renders a "contains" LIKE pattern with the user's own % and _
// matched literally.
func likeQuote(s string) string {
	q := Quote(s)
	inner := strings.NewReplacer("%", `\%`, "_", `\_`).Replace(q[1 : len(q)-1])
	return "'%" + inner + "%'"
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}
