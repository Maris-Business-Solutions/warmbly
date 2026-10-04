package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// CRMProvider names the system of record for a workspace's CRM data.
type CRMProvider string

const (
	CRMProviderNative    CRMProvider = "native"
	CRMProviderHubSpot   CRMProvider = "hubspot"
	CRMProviderPipedrive CRMProvider = "pipedrive"
)

// External reports a provider other than Warmbly's own CRM.
func (p CRMProvider) External() bool {
	return p == CRMProviderHubSpot || p == CRMProviderPipedrive
}

// CRM object types a provider record can mirror (crm_external_links.object_type).
const (
	CRMObjectPipeline = "pipeline"
	CRMObjectStage    = "stage"
	CRMObjectDeal     = "deal"
	CRMObjectTask     = "task"
	CRMObjectNote     = "note"
	CRMObjectMeeting  = "meeting"
	// CRMObjectEmail links a campaign send (local task id) to the logged email.
	CRMObjectEmail = "email"
)

// CRM sync job kinds (crm_sync_jobs.kind).
const (
	CRMJobLogEmail      = "log_email"
	CRMJobLogEvent      = "log_event"
	CRMJobContactProps  = "contact_props"
	CRMJobPushContact   = "push_contact"
	CRMJobPushDeal      = "push_deal"
	CRMJobPushTask      = "push_task"
	CRMJobPushNote      = "push_note"
	CRMJobLogMeeting    = "log_meeting"
	CRMJobReplyOutcome  = "reply_outcome"
	CRMJobRefreshObject = "refresh_object"
	CRMJobBackfill      = "backfill"
)

// CRMActivityLog chooses which Warmbly activity is written to the provider's
// timeline. Opens and clicks default off: machine opens make them noise.
type CRMActivityLog struct {
	Sent         bool `json:"sent"`
	Replies      bool `json:"replies"`
	Bounces      bool `json:"bounces"`
	Unsubscribes bool `json:"unsubscribes"`
	Opens        bool `json:"opens"`
	Clicks       bool `json:"clicks"`
	Meetings     bool `json:"meetings"`
}

// CRMReplyOutcome is what a positive ("interested") reply does in the provider.
type CRMReplyOutcome struct {
	// LeadStatus is written to the contact's lead status ("" leaves it alone).
	LeadStatus string `json:"lead_status"`
	// LifecycleStage is written to the contact's lifecycle stage ("" leaves it).
	LifecycleStage string `json:"lifecycle_stage"`
	CreateDeal     bool   `json:"create_deal"`
	// CreateLead adds the person to Pipedrive's Leads Inbox (Pipedrive only).
	CreateLead bool `json:"create_lead,omitempty"`
	// DealPipelineID and DealStageID are local (mirrored) ids.
	DealPipelineID *uuid.UUID `json:"deal_pipeline_id,omitempty"`
	DealStageID    *uuid.UUID `json:"deal_stage_id,omitempty"`
}

// CRMExitRules stop a contact's campaigns when the provider says they have
// moved on, so nobody is followed up after becoming a deal or a customer.
type CRMExitRules struct {
	DealCreated bool `json:"deal_created"`
	// LifecycleStages stops a contact whose lifecycle stage becomes any of these.
	LifecycleStages []string `json:"lifecycle_stages"`
	OptedOut        bool     `json:"opted_out"`
}

// CRMEnrollmentGuards skip contacts at import and enrolment time.
type CRMEnrollmentGuards struct {
	SkipLifecycleStages []string `json:"skip_lifecycle_stages"`
	SkipOpenDeals       bool     `json:"skip_open_deals"`
	SkipOtherOwners     bool     `json:"skip_other_owners"`
	SkipOptedOut        bool     `json:"skip_opted_out"`
}

// CRMProviderConfig is crm_settings.config: every choice the setup wizard asks,
// with defaults that work without changing anything.
type CRMProviderConfig struct {
	Activity CRMActivityLog `json:"activity"`
	// CreateContacts lets Warmbly create a provider contact for someone it emails
	// who is not in the CRM yet.
	CreateContacts bool `json:"create_contacts"`
	// CreateCompanies creates and associates a company from the contact's domain.
	CreateCompanies bool `json:"create_companies"`
	// WriteProperties keeps the "Warmbly" property group on contacts current.
	WriteProperties bool                `json:"write_properties"`
	PositiveReply   CRMReplyOutcome     `json:"positive_reply"`
	ExitRules       CRMExitRules        `json:"exit_rules"`
	Guards          CRMEnrollmentGuards `json:"guards"`
	// DealPipelines limits the mirrored deals to these provider pipeline ids
	// (empty: every pipeline).
	DealPipelines []string `json:"deal_pipelines"`
	// DisplayProperties are extra contact properties shown in the dashboard.
	DisplayProperties []string `json:"display_properties"`
	// FieldMap maps a Warmbly contact field (first_name, company, custom:x) to
	// a provider property; FieldDirection says which side wins per field.
	FieldMap       map[string]string `json:"field_map"`
	FieldDirection map[string]string `json:"field_direction"`
	// For is the provider these choices were made for ("" is HubSpot, the
	// only one before Pipedrive), so a switch never reads another's ids.
	For CRMProvider `json:"for,omitempty"`
}

// BelongsTo reports whether the choices were made for provider p.
func (c *CRMProviderConfig) BelongsTo(p CRMProvider) bool {
	if c.For == "" {
		return p == CRMProviderHubSpot
	}
	return c.For == p
}

// Field directions for CRMProviderConfig.FieldDirection.
const (
	CRMFieldPush = "push" // Warmbly writes the provider
	CRMFieldPull = "pull" // the provider writes Warmbly
	CRMFieldBoth = "both" // the most recent change wins
)

// DefaultCRMProviderConfigFor is the zero-question setup for one provider.
// Pipedrive has no lifecycle stages: its person labels stand in for them, and
// their ids are only known once connected, so they are seeded at the switch.
func DefaultCRMProviderConfigFor(p CRMProvider) CRMProviderConfig {
	if p != CRMProviderPipedrive {
		return DefaultCRMProviderConfig()
	}
	return CRMProviderConfig{
		Activity: CRMActivityLog{
			Sent: true, Replies: true, Bounces: true, Unsubscribes: true, Meetings: true,
		},
		CreateContacts:  true,
		CreateCompanies: true,
		WriteProperties: true,
		ExitRules:       CRMExitRules{DealCreated: true, LifecycleStages: []string{}, OptedOut: true},
		Guards:          CRMEnrollmentGuards{SkipLifecycleStages: []string{}, SkipOpenDeals: true, SkipOptedOut: true},
		FieldMap: map[string]string{
			"first_name": "first_name",
			"last_name":  "last_name",
			"company":    "org_name",
			"phone":      "phone",
		},
		FieldDirection: map[string]string{
			"first_name": CRMFieldBoth,
			"last_name":  CRMFieldBoth,
			"company":    CRMFieldBoth,
			"phone":      CRMFieldBoth,
		},
	}
}

// DefaultCRMProviderConfig is the zero-question setup.
func DefaultCRMProviderConfig() CRMProviderConfig {
	return CRMProviderConfig{
		Activity: CRMActivityLog{
			Sent: true, Replies: true, Bounces: true, Unsubscribes: true, Meetings: true,
		},
		CreateContacts:  true,
		CreateCompanies: true,
		WriteProperties: true,
		PositiveReply: CRMReplyOutcome{
			LeadStatus: "IN_PROGRESS",
		},
		ExitRules: CRMExitRules{
			DealCreated:     true,
			LifecycleStages: []string{"opportunity", "customer"},
			OptedOut:        true,
		},
		Guards: CRMEnrollmentGuards{
			SkipLifecycleStages: []string{"customer", "evangelist"},
			SkipOpenDeals:       true,
			SkipOptedOut:        true,
		},
		FieldMap: map[string]string{
			"first_name": "firstname",
			"last_name":  "lastname",
			"company":    "company",
			"phone":      "phone",
		},
		FieldDirection: map[string]string{
			"first_name": CRMFieldBoth,
			"last_name":  CRMFieldBoth,
			"company":    CRMFieldBoth,
			"phone":      CRMFieldBoth,
		},
	}
}

// Validate bounds the config before it is stored.
func (c *CRMProviderConfig) Validate() error {
	if len(c.DealPipelines) > 50 || len(c.DisplayProperties) > 40 || len(c.FieldMap) > 100 {
		return fmt.Errorf("too many entries")
	}
	if len(c.ExitRules.LifecycleStages) > 20 || len(c.Guards.SkipLifecycleStages) > 20 {
		return fmt.Errorf("too many lifecycle stages")
	}
	for k, v := range c.FieldMap {
		if !validCRMField(k) {
			return fmt.Errorf("unknown Warmbly field %q", k)
		}
		if !validProviderProperty(v) {
			return fmt.Errorf("invalid property name %q", v)
		}
	}
	for k, v := range c.FieldDirection {
		if _, ok := c.FieldMap[k]; !ok {
			return fmt.Errorf("direction set for unmapped field %q", k)
		}
		switch v {
		case CRMFieldPush, CRMFieldPull, CRMFieldBoth:
		default:
			return fmt.Errorf("invalid direction %q", v)
		}
	}
	for _, p := range append(append([]string{}, c.DisplayProperties...), c.DealPipelines...) {
		if !validProviderProperty(p) {
			return fmt.Errorf("invalid name %q", p)
		}
	}
	for _, s := range append(append([]string{}, c.ExitRules.LifecycleStages...), c.Guards.SkipLifecycleStages...) {
		if !validProviderProperty(s) {
			return fmt.Errorf("invalid lifecycle stage %q", s)
		}
	}
	if !validProviderValue(c.PositiveReply.LeadStatus) || !validProviderValue(c.PositiveReply.LifecycleStage) {
		return fmt.Errorf("invalid reply outcome value")
	}
	if c.PositiveReply.CreateDeal && (c.PositiveReply.DealPipelineID == nil || c.PositiveReply.DealStageID == nil) {
		return fmt.Errorf("pick a pipeline and stage for deals created on a positive reply")
	}
	return nil
}

// CRMContactFields are the Warmbly contact fields a provider property can map to.
var CRMContactFields = []string{"first_name", "last_name", "email", "company", "phone"}

func validCRMField(k string) bool {
	if strings.HasPrefix(k, "custom:") {
		return len(k) > len("custom:") && len(k) <= 128
	}
	for _, f := range CRMContactFields {
		if f == k {
			return true
		}
	}
	return false
}

// validProviderProperty accepts the internal names providers use for
// properties, pipelines and enum values.
func validProviderProperty(v string) bool {
	if v == "" || len(v) > 100 {
		return false
	}
	for _, r := range v {
		if !(r == '_' || r == '-' || r == '.' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

func validProviderValue(v string) bool { return v == "" || validProviderProperty(v) }

// CRMSettings is the workspace's CRM mode as the dashboard reads it.
type CRMSettings struct {
	OrganizationID   uuid.UUID         `json:"organization_id"`
	Provider         CRMProvider       `json:"provider"`
	ConnectionID     *uuid.UUID        `json:"connection_id,omitempty"`
	Config           CRMProviderConfig `json:"config"`
	SetupCompletedAt *time.Time        `json:"setup_completed_at,omitempty"`
	UpdatedAt        time.Time         `json:"updated_at"`
	// Read-only facts about the connected account, for links and headers.
	Account *CRMAccount `json:"account,omitempty"`
}

// CRMAccount identifies the connected provider account.
type CRMAccount struct {
	ExternalID string `json:"external_id"`
	Name       string `json:"name"`
	// AppURL is the provider's web app root for this account, for "Open in" links.
	AppURL string `json:"app_url"`
	Status string `json:"status"`
	Health string `json:"health"`
	// MissingScopes lists permissions the connection lacks for full mode, so
	// the dashboard can ask for a reconnect.
	MissingScopes []string `json:"missing_scopes,omitempty"`
}

// UpdateCRMSettings is the body of PUT /crm/settings.
type UpdateCRMSettings struct {
	Provider     *CRMProvider       `json:"provider,omitempty"`
	ConnectionID *uuid.UUID         `json:"connection_id,omitempty"`
	Config       *CRMProviderConfig `json:"config,omitempty"`
	// CompleteSetup stamps the wizard as finished.
	CompleteSetup bool `json:"complete_setup,omitempty"`
}

// CRMExternalRef says where a CRM record lives outside Warmbly. Attached to
// deals, tasks, notes and pipelines read in provider mode.
type CRMExternalRef struct {
	Provider   CRMProvider `json:"provider"`
	ExternalID string      `json:"external_id"`
	URL        string      `json:"url,omitempty"`
	SyncedAt   time.Time   `json:"synced_at"`
	// OwnerName is the provider owner when that owner is not a workspace member.
	OwnerName string `json:"owner_name,omitempty"`
}

// CRMExternalLink is one crm_external_links row.
type CRMExternalLink struct {
	OrganizationID uuid.UUID
	Provider       CRMProvider
	ObjectType     string
	LocalID        uuid.UUID
	ExternalID     string
	Meta           map[string]any
	SyncedAt       time.Time
}

// CRMContactRecord is the provider's view of one Warmbly contact.
type CRMContactRecord struct {
	OrganizationID    uuid.UUID         `json:"organization_id"`
	ContactID         uuid.UUID         `json:"contact_id"`
	Provider          CRMProvider       `json:"provider"`
	ExternalID        string            `json:"external_id"`
	OwnerExternalID   string            `json:"owner_external_id,omitempty"`
	LifecycleStage    string            `json:"lifecycle_stage,omitempty"`
	LeadStatus        string            `json:"lead_status,omitempty"`
	CompanyExternalID string            `json:"company_external_id,omitempty"`
	CompanyName       string            `json:"company_name,omitempty"`
	CompanyDomain     string            `json:"company_domain,omitempty"`
	OptedOut          bool              `json:"opted_out"`
	Properties        map[string]string `json:"properties"`
	ExternalUpdatedAt *time.Time        `json:"external_updated_at,omitempty"`
	SyncedAt          time.Time         `json:"synced_at"`
}

// CRMContactView is what the contact drawer and the inbox panel show for a
// contact in provider mode.
type CRMContactView struct {
	Provider       CRMProvider       `json:"provider"`
	Linked         bool              `json:"linked"`
	ExternalID     string            `json:"external_id,omitempty"`
	URL            string            `json:"url,omitempty"`
	Owner          *CRMOwner         `json:"owner,omitempty"`
	LifecycleStage *CRMOption        `json:"lifecycle_stage,omitempty"`
	LeadStatus     *CRMOption        `json:"lead_status,omitempty"`
	Company        *CRMCompanyRef    `json:"company,omitempty"`
	OptedOut       bool              `json:"opted_out"`
	Properties     []CRMPropertyView `json:"properties"`
	SyncedAt       *time.Time        `json:"synced_at,omitempty"`
}

// CRMCompanyRef is a contact's primary company in the provider.
type CRMCompanyRef struct {
	ExternalID string `json:"external_id"`
	Name       string `json:"name"`
	Domain     string `json:"domain,omitempty"`
	URL        string `json:"url,omitempty"`
}

// CRMPropertyView is one displayed provider property, labelled.
type CRMPropertyView struct {
	Name  string `json:"name"`
	Label string `json:"label"`
	Value string `json:"value"`
}

// CRMOption is a provider enum value with its label.
type CRMOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// CRMOwner is a provider user who can own records.
type CRMOwner struct {
	ExternalID string     `json:"external_id"`
	Email      string     `json:"email"`
	FirstName  string     `json:"first_name"`
	LastName   string     `json:"last_name"`
	UserID     *uuid.UUID `json:"user_id,omitempty"`
	UserPinned bool       `json:"user_pinned"`
	Archived   bool       `json:"archived"`
}

// DisplayName is the owner's name, or the address when unnamed.
func (o CRMOwner) DisplayName() string {
	n := strings.TrimSpace(o.FirstName + " " + o.LastName)
	if n == "" {
		return o.Email
	}
	return n
}

// UpdateCRMContact is the body of PATCH /crm/contacts/:id/record: the sales
// fields edited in place from the inbox panel or the contact drawer.
type UpdateCRMContact struct {
	OwnerExternalID *string `json:"owner_external_id,omitempty"`
	LifecycleStage  *string `json:"lifecycle_stage,omitempty"`
	LeadStatus      *string `json:"lead_status,omitempty"`
}

// MapCRMOwner is the body of PUT /crm/owners/:externalId.
type MapCRMOwner struct {
	UserID *uuid.UUID `json:"user_id"`
}

// CRMSyncJob is one crm_sync_jobs row.
type CRMSyncJob struct {
	ID             uuid.UUID      `json:"id"`
	OrganizationID uuid.UUID      `json:"organization_id"`
	Provider       CRMProvider    `json:"provider"`
	Kind           string         `json:"kind"`
	DedupeKey      string         `json:"-"`
	LeaseToken     uuid.UUID      `json:"-"`
	Subject        string         `json:"subject"`
	Payload        map[string]any `json:"-"`
	Status         string         `json:"status"`
	Attempts       int            `json:"attempts"`
	NextAttemptAt  time.Time      `json:"next_attempt_at"`
	LastError      string         `json:"last_error,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	FinishedAt     *time.Time     `json:"finished_at,omitempty"`
}

// CRMSyncHealth is the sync health panel: counts, failures and freshness.
type CRMSyncHealth struct {
	Pending      int64           `json:"pending"`
	Failed       int64           `json:"failed"`
	Done24h      int64           `json:"done_24h"`
	LastSyncedAt *time.Time      `json:"last_synced_at,omitempty"`
	Cursors      []CRMSyncCursor `json:"cursors"`
	Failures     []CRMSyncJob    `json:"failures"`
	Counts       CRMMirrorCounts `json:"counts"`
}

// CRMSyncCursor reports one incremental pull.
type CRMSyncCursor struct {
	ObjectType string     `json:"object_type"`
	CursorAt   *time.Time `json:"cursor_at,omitempty"`
	LastRunAt  *time.Time `json:"last_run_at,omitempty"`
	LastError  string     `json:"last_error,omitempty"`
}

// CRMMirrorCounts is how much provider data the workspace mirrors.
type CRMMirrorCounts struct {
	Contacts  int64 `json:"contacts"`
	Deals     int64 `json:"deals"`
	Tasks     int64 `json:"tasks"`
	Pipelines int64 `json:"pipelines"`
	Owners    int64 `json:"owners"`
}

// CRMMetadata is the provider vocabulary the pickers render: lifecycle stages,
// lead statuses, task types and properties.
type CRMMetadata struct {
	LifecycleStages []CRMOption   `json:"lifecycle_stages"`
	LeadStatuses    []CRMOption   `json:"lead_statuses"`
	TaskTypes       []CRMOption   `json:"task_types"`
	Properties      []CRMProperty `json:"properties"`
	Pipelines       []CRMOption   `json:"pipelines"`
}

// CRMProperty is a provider contact property a field can map to.
type CRMProperty struct {
	Name      string `json:"name"`
	Label     string `json:"label"`
	Type      string `json:"type"`
	GroupName string `json:"group_name"`
	ReadOnly  bool   `json:"read_only"`
}

// CRMList is a provider contact list Warmbly can import from.
type CRMList struct {
	ExternalID string `json:"external_id"`
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	Dynamic    bool   `json:"dynamic"`
	UpdatedAt  string `json:"updated_at,omitempty"`
}

// CRMListsResult is a page of provider lists.
type CRMListsResult struct {
	Data       []CRMList  `json:"data"`
	Pagination Pagination `json:"pagination"`
}

// CRMImportRequest is the body of the list preview and import.
type CRMImportRequest struct {
	ListID string `json:"list_id" binding:"required,max=64"`
	// ApplyGuards drops contacts the workspace's enrolment guards exclude.
	ApplyGuards bool `json:"apply_guards"`
}

// CRMImportPreview counts who a list import would bring in and who it skips.
type CRMImportPreview struct {
	ListName string            `json:"list_name"`
	Total    int               `json:"total"`
	Included int               `json:"included"`
	Skipped  []CRMImportSkip   `json:"skipped"`
	Sample   []CRMImportPerson `json:"sample"`
	// Truncated is set when the list is larger than one import takes.
	Truncated bool `json:"truncated"`
}

// CRMImportSkip is one skip reason and how many contacts it removed.
type CRMImportSkip struct {
	Reason string `json:"reason"`
	Label  string `json:"label"`
	Count  int    `json:"count"`
}

// CRMImportPerson is a sample row of the preview.
type CRMImportPerson struct {
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Company   string `json:"company"`
}

// CRMImportResult names the contact import draft a list import produced; the
// dashboard finishes it in the regular import review.
type CRMImportResult struct {
	ImportID uuid.UUID        `json:"import_id"`
	Preview  CRMImportPreview `json:"preview"`
}

// CRMBackfillRequest copies Warmbly's own CRM data into the provider once,
// when a workspace switches.
type CRMBackfillRequest struct {
	Deals bool `json:"deals"`
	Tasks bool `json:"tasks"`
	Notes bool `json:"notes"`
}

// CRMBackfillPreview counts what a backfill would copy.
type CRMBackfillPreview struct {
	Deals int64 `json:"deals"`
	Tasks int64 `json:"tasks"`
	Notes int64 `json:"notes"`
}
