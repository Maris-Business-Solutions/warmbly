package pipedrive

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/warmbly/warmbly/internal/app/crm"
	"github.com/warmbly/warmbly/internal/app/crmmode"
	"github.com/warmbly/warmbly/internal/app/integration"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/infrastructure/cache"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

const provider = models.CRMProviderPipedrive

var (
	_ crm.External     = (*Service)(nil)
	_ crmmode.Provider = (*Service)(nil)
)

// Tokens hands out a connection's current OAuth token (the integration service).
type Tokens interface {
	AccessToken(ctx context.Context, orgID, connID uuid.UUID) (string, *models.IntegrationConnection, error)
	GetConnection(ctx context.Context, orgID, id uuid.UUID) (*models.IntegrationConnection, error)
	MarkConnectionHealth(ctx context.Context, connID uuid.UUID, status models.IntegrationStatus, health models.IntegrationHealth, detail string)
	MergeConnectionDisplay(ctx context.Context, connID uuid.UUID, patch map[string]any) error
}

// Contacts reads Warmbly contacts.
type Contacts interface {
	GetByIDsAndOrganization(ctx context.Context, organizationID uuid.UUID, ids []uuid.UUID) ([]models.Contact, *errx.Error)
	GetByEmailAndOrganization(ctx context.Context, organizationID uuid.UUID, email string) (*models.Contact, *errx.Error)
}

// LeadHolder parks a contact's campaigns (the CRM exit rules).
type LeadHolder interface {
	HoldLeadEverywhere(ctx context.Context, contactID uuid.UUID, until *time.Time, reason, source string) ([]uuid.UUID, error)
}

// Suppressor adds an address to the workspace suppression list.
type Suppressor interface {
	UpsertSuppressedRecipient(ctx context.Context, entry *models.SuppressedRecipient) error
}

// Importer starts a contact import draft from CSV.
type Importer interface {
	Create(ctx context.Context, orgID, userID uuid.UUID, r io.Reader, filename string) (*models.ContactImport, *errx.Error)
}

// Realtime tells the dashboard mirrored data moved.
type Realtime interface {
	PublishCRMSynced(ctx context.Context, orgID uuid.UUID, objects []string, contactID string)
}

// Deps are the Service's collaborators. Importer, Leads, Holds, Suppress and
// Realtime may be nil in processes that do not need them.
type Deps struct {
	Repo     repository.CRMProviderRepository
	CRM      repository.CRMRepository
	Tokens   Tokens
	Contacts Contacts
	Holds    LeadHolder
	Suppress Suppressor
	Importer Importer
	Leads    Leads
	Realtime Realtime
	Cache    *cache.Cache
	// AppURL is the dashboard origin, for the "Open in Warmbly" field.
	AppURL string
	// PublicURL is the backend's public origin, where Pipedrive delivers webhooks.
	PublicURL string
	// ClientSecret derives each connection's webhook password
	// (PIPEDRIVE_OAUTH_CLIENT_SECRET).
	ClientSecret string
}

// Service is the Pipedrive CRM mode for every workspace on the instance.
type Service struct {
	d Deps

	mu       sync.Mutex
	settings map[uuid.UUID]cachedSettings
	clients  map[uuid.UUID]*orgClient
}

type orgClient struct {
	*Client
	connID uuid.UUID
	base   string
	// probed is when a failed host discovery fell back to the shared host;
	// zero once the company host is known.
	probed time.Time
}

// discoverAgain spaces out host discovery after it failed.
const discoverAgain = 5 * time.Minute

type cachedSettings struct {
	row *repository.CRMSettingsRow
	at  time.Time
}

const settingsTTL = 30 * time.Second

func New(d Deps) *Service {
	return &Service{d: d, settings: map[uuid.UUID]cachedSettings{}, clients: map[uuid.UUID]*orgClient{}}
}

// Name is the CRM this service runs.
func (s *Service) Name() models.CRMProvider { return provider }

// Integration is the connection type Pipedrive mode runs on.
func (s *Service) Integration() models.IntegrationProvider { return models.IntegrationPipedrive }

// org is a workspace in Pipedrive mode, resolved for one operation.
type org struct {
	ID       uuid.UUID
	ConnID   uuid.UUID
	Company  string
	Domain   string
	Config   models.CRMProviderConfig
	Client   *Client
	Settings *repository.CRMSettingsRow
}

func (s *Service) settingsRow(ctx context.Context, orgID uuid.UUID) (*repository.CRMSettingsRow, error) {
	s.mu.Lock()
	if c, ok := s.settings[orgID]; ok && time.Since(c.at) < settingsTTL {
		s.mu.Unlock()
		return c.row, nil
	}
	s.mu.Unlock()
	row, err := s.d.Repo.GetSettings(ctx, orgID)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.settings[orgID] = cachedSettings{row: row, at: time.Now()}
	s.mu.Unlock()
	return row, nil
}

// Forget drops this process's cached view of a workspace's mode.
func (s *Service) Forget(orgID uuid.UUID) {
	s.mu.Lock()
	delete(s.settings, orgID)
	delete(s.clients, orgID)
	s.mu.Unlock()
}

// Active reports whether the workspace runs its CRM on Pipedrive.
func (s *Service) Active(ctx context.Context, orgID uuid.UUID) bool {
	row, err := s.settingsRow(ctx, orgID)
	return err == nil && row != nil && row.Provider == provider && row.ConnectionID != nil
}

var errNotConnected = errors.New("pipedrive is not connected")

// resolve returns the workspace's Pipedrive context, or nil when it is not in
// Pipedrive mode.
func (s *Service) resolve(ctx context.Context, orgID uuid.UUID) (*org, error) {
	row, err := s.settingsRow(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if row == nil || row.Provider != provider || row.ConnectionID == nil {
		return nil, nil
	}
	conn, err := s.d.Tokens.GetConnection(ctx, orgID, *row.ConnectionID)
	if err != nil || conn == nil {
		return nil, errNotConnected
	}
	return s.orgFor(ctx, orgID, conn, row), nil
}

func (s *Service) orgFor(ctx context.Context, orgID uuid.UUID, conn *models.IntegrationConnection, row *repository.CRMSettingsRow) *org {
	o := &org{
		ID:       orgID,
		ConnID:   conn.ID,
		Company:  displayString(conn.DisplayFields, "company_id"),
		Domain:   displayString(conn.DisplayFields, "company_domain"),
		Settings: row,
	}
	if row != nil {
		o.Config = row.Config
	}
	s.mu.Lock()
	cached := s.clients[orgID]
	s.mu.Unlock()
	base, _ := integration.PipedriveAPIDomain(displayString(conn.DisplayFields, "api_domain"))
	var probed time.Time
	if base == "" && cached != nil && cached.connID == conn.ID &&
		(cached.probed.IsZero() || time.Since(cached.probed) < discoverAgain) {
		base, probed = cached.base, cached.probed
	}
	if base == "" {
		var found bool
		if base, found = s.discoverDomain(ctx, orgID, conn); !found {
			probed = time.Now()
		}
	}
	if o.Domain == "" {
		o.Domain = domainOf(base)
	}
	if o.Company == "" {
		o.Company = "conn-" + conn.ID.String()
	}
	s.mu.Lock()
	cl := s.clients[orgID]
	if cl == nil || cl.connID != conn.ID || cl.base != base || cl.probed != probed {
		connID := conn.ID
		cl = &orgClient{Client: NewClient(o.Company, base, func(ctx context.Context) (string, error) {
			tok, _, err := s.d.Tokens.AccessToken(ctx, orgID, connID)
			return tok, err
		}, s.d.Cache), connID: connID, base: base, probed: probed}
		s.clients[orgID] = cl
	}
	s.mu.Unlock()
	o.Client = cl.Client
	return o
}

// discoverDomain finds the company host of a connection made before Warmbly
// recorded it, and remembers it on the connection.
func (s *Service) discoverDomain(ctx context.Context, orgID uuid.UUID, conn *models.IntegrationConnection) (string, bool) {
	const fallback = "https://api.pipedrive.com"
	connID := conn.ID
	probe := NewClient("conn-"+connID.String(), fallback, func(ctx context.Context) (string, error) {
		tok, _, err := s.d.Tokens.AccessToken(ctx, orgID, connID)
		return tok, err
	}, s.d.Cache)
	me, err := probe.Me(ctx)
	if err != nil || me.CompanyDomain == "" {
		return fallback, false
	}
	base, err := integration.PipedriveAPIDomain("https://" + strings.ToLower(me.CompanyDomain) + ".pipedrive.com")
	if err != nil {
		return fallback, false
	}
	_ = s.d.Tokens.MergeConnectionDisplay(ctx, connID, map[string]any{
		"api_domain": base, "company_domain": strings.ToLower(me.CompanyDomain), "company_id": id(me.CompanyID),
	})
	return base, true
}

// domainOf is the company subdomain of an API host.
func domainOf(base string) string {
	host := strings.TrimPrefix(base, "https://")
	sub, _, _ := strings.Cut(host, ".")
	if sub == "api" {
		return ""
	}
	return sub
}

// mustResolve is resolve for paths that only run in Pipedrive mode.
func (s *Service) mustResolve(ctx context.Context, orgID uuid.UUID) (*org, *errx.Error) {
	o, err := s.resolve(ctx, orgID)
	if err != nil {
		return nil, s.userError(ctx, nil, err)
	}
	if o == nil {
		return nil, errx.NewWithIdentifier(errx.Conflict, "crm_not_connected", "Connect Pipedrive and choose it as your CRM first.")
	}
	return o, nil
}

// userError turns a provider failure into the answer a person can act on, and
// flags the connection when Pipedrive stopped trusting the token.
func (s *Service) userError(ctx context.Context, o *org, err error) *errx.Error {
	if err == nil {
		return nil
	}
	var xe *errx.Error
	if errors.As(err, &xe) {
		return xe
	}
	if errors.Is(err, errNotConnected) {
		return errx.NewWithIdentifier(errx.Conflict, "crm_reauth_required", "Pipedrive is disconnected. Reconnect it in Integrations > Pipedrive.")
	}
	if ae, ok := AsAPIError(err); ok {
		switch {
		case ae.Status == 401:
			if o != nil {
				s.d.Tokens.MarkConnectionHealth(ctx, o.ConnID, models.IntegrationStatusReauthRequired, models.IntegrationHealthDown,
					"Pipedrive refused the token: reconnect required")
			}
			return errx.NewWithIdentifier(errx.Conflict, "crm_reauth_required",
				"Reconnect Pipedrive: Warmbly's access was revoked.")
		case ae.Status == 403:
			return errx.NewWithIdentifier(errx.Conflict, "crm_reauth_required",
				"Pipedrive refused this for the connected user. Reconnect Pipedrive as a user who can do it, or ask a Pipedrive admin.")
		case ae.Status == 402:
			return errx.NewWithIdentifier(errx.Unprocessable, "crm_provider_rejected",
				"Your Pipedrive plan does not include this feature.")
		case ae.Retryable():
			return errx.NewWithIdentifier(errx.ServiceUnavailable, "crm_unavailable", "Pipedrive did not answer. Try again in a moment.")
		case ae.NotFound():
			return errx.NewWithIdentifier(errx.NotFound, "crm_record_missing", "This record no longer exists in Pipedrive.")
		default:
			return errx.NewWithIdentifier(errx.Unprocessable, "crm_provider_rejected", "Pipedrive refused the change: "+ae.Message)
		}
	}
	if strings.Contains(err.Error(), "token refresh failed") || strings.Contains(err.Error(), "connection not found") {
		return errx.NewWithIdentifier(errx.Conflict, "crm_reauth_required", "Reconnect Pipedrive: Warmbly's access was revoked.")
	}
	log.Warn().Err(err).Msg("pipedrive: call failed")
	return errx.NewWithIdentifier(errx.ServiceUnavailable, "crm_unavailable", "Pipedrive did not answer. Try again in a moment.")
}

func (s *Service) notify(ctx context.Context, orgID uuid.UUID, contactID string, objects ...string) {
	if s.d.Realtime != nil {
		s.d.Realtime.PublishCRMSynced(ctx, orgID, objects, contactID)
	}
}

func displayString(raw []byte, key string) string {
	if len(raw) == 0 {
		return ""
	}
	m := map[string]any{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	switch v := m[key].(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return ""
}

// ---------- links into Pipedrive ----------

func (o *org) web() string {
	if o.Domain == "" {
		return "https://app.pipedrive.com"
	}
	return "https://" + o.Domain + ".pipedrive.com"
}

func (o *org) personURL(ext string) string {
	if ext == "" {
		return ""
	}
	return o.web() + "/person/" + ext
}

func (o *org) dealURL(ext string) string {
	if ext == "" {
		return ""
	}
	return o.web() + "/deal/" + ext
}

func (o *org) orgURL(ext string) string {
	if ext == "" {
		return ""
	}
	return o.web() + "/organization/" + ext
}

func (o *org) pipelineURL(ext string) string {
	if ext == "" {
		return o.web() + "/pipeline"
	}
	return o.web() + "/pipeline/" + ext
}

func (o *org) activitiesURL() string { return o.web() + "/activities/list" }

// ---------- ids ----------

// extID parses a Pipedrive id stored as text; 0 when it is not one.
func extID(v string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func ptrID(p *int64) string {
	if p == nil || *p == 0 {
		return ""
	}
	return id(*p)
}

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
