package pipedrive

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

// Records as API v2 returns them (notes, users, filters, activity types and
// webhooks are v1 only).

type Me struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Email         string `json:"email"`
	CompanyID     int64  `json:"company_id"`
	CompanyName   string `json:"company_name"`
	CompanyDomain string `json:"company_domain"`
}

type User struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Email      string `json:"email"`
	ActiveFlag bool   `json:"active_flag"`
	IsDeleted  bool   `json:"is_deleted"`
}

type Pipeline struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	OrderNr   int    `json:"order_nr"`
	IsDeleted bool   `json:"is_deleted"`
}

type Stage struct {
	ID              int64    `json:"id"`
	Name            string   `json:"name"`
	OrderNr         int      `json:"order_nr"`
	PipelineID      int64    `json:"pipeline_id"`
	DealProbability *float64 `json:"deal_probability"`
	IsDeleted       bool     `json:"is_deleted"`
}

type Deal struct {
	ID                int64    `json:"id"`
	Title             string   `json:"title"`
	OwnerID           *int64   `json:"owner_id"`
	PersonID          *int64   `json:"person_id"`
	OrgID             *int64   `json:"org_id"`
	PipelineID        int64    `json:"pipeline_id"`
	StageID           int64    `json:"stage_id"`
	Value             *float64 `json:"value"`
	Currency          string   `json:"currency"`
	Status            string   `json:"status"`
	LostReason        *string  `json:"lost_reason"`
	ExpectedCloseDate *string  `json:"expected_close_date"`
	CloseTime         *string  `json:"close_time"`
	WonTime           *string  `json:"won_time"`
	LostTime          *string  `json:"lost_time"`
	AddTime           string   `json:"add_time"`
	UpdateTime        string   `json:"update_time"`
	IsDeleted         bool     `json:"is_deleted"`
}

// ContactPoint is one of a person's emails or phones.
type ContactPoint struct {
	Value   string `json:"value"`
	Primary bool   `json:"primary"`
	Label   string `json:"label,omitempty"`
}

type Person struct {
	ID              int64          `json:"id"`
	Name            string         `json:"name"`
	FirstName       string         `json:"first_name"`
	LastName        string         `json:"last_name"`
	OwnerID         *int64         `json:"owner_id"`
	OrgID           *int64         `json:"org_id"`
	Emails          []ContactPoint `json:"emails"`
	Phones          []ContactPoint `json:"phones"`
	LabelIDs        []int64        `json:"label_ids"`
	CustomFields    map[string]any `json:"custom_fields"`
	MarketingStatus string         `json:"marketing_status"`
	AddTime         string         `json:"add_time"`
	UpdateTime      string         `json:"update_time"`
	IsDeleted       bool           `json:"is_deleted"`
}

// Email is the person's primary address, else the first.
func (p *Person) Email() string {
	for _, e := range p.Emails {
		if e.Primary && strings.TrimSpace(e.Value) != "" {
			return strings.ToLower(strings.TrimSpace(e.Value))
		}
	}
	for _, e := range p.Emails {
		if strings.TrimSpace(e.Value) != "" {
			return strings.ToLower(strings.TrimSpace(e.Value))
		}
	}
	return ""
}

// Phone is the person's primary phone, else the first.
func (p *Person) Phone() string {
	for _, e := range p.Phones {
		if e.Primary && e.Value != "" {
			return e.Value
		}
	}
	if len(p.Phones) > 0 {
		return p.Phones[0].Value
	}
	return ""
}

type Organization struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Website   string `json:"website"`
	IsDeleted bool   `json:"is_deleted"`
}

type Participant struct {
	PersonID int64 `json:"person_id"`
	Primary  bool  `json:"primary"`
}

type Activity struct {
	ID               int64         `json:"id"`
	Subject          string        `json:"subject"`
	Type             string        `json:"type"`
	OwnerID          *int64        `json:"owner_id"`
	DealID           *int64        `json:"deal_id"`
	PersonID         *int64        `json:"person_id"`
	OrgID            *int64        `json:"org_id"`
	DueDate          string        `json:"due_date"`
	DueTime          string        `json:"due_time"`
	Done             bool          `json:"done"`
	MarkedAsDoneTime string        `json:"marked_as_done_time"`
	Note             string        `json:"note"`
	Participants     []Participant `json:"participants"`
	AddTime          string        `json:"add_time"`
	UpdateTime       string        `json:"update_time"`
	IsDeleted        bool          `json:"is_deleted"`
}

// Person is the activity's primary participant.
func (a *Activity) Person() int64 {
	for _, p := range a.Participants {
		if p.Primary {
			return p.PersonID
		}
	}
	if a.PersonID != nil {
		return *a.PersonID
	}
	if len(a.Participants) > 0 {
		return a.Participants[0].PersonID
	}
	return 0
}

type ActivityType struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	KeyString  string `json:"key_string"`
	Color      string `json:"color"`
	OrderNr    int    `json:"order_nr"`
	ActiveFlag bool   `json:"active_flag"`
}

type Note struct {
	ID         int64  `json:"id"`
	Content    string `json:"content"`
	DealID     *int64 `json:"deal_id"`
	PersonID   *int64 `json:"person_id"`
	OrgID      *int64 `json:"org_id"`
	UserID     int64  `json:"user_id"`
	AddTime    string `json:"add_time"`
	UpdateTime string `json:"update_time"`
	ActiveFlag bool   `json:"active_flag"`
}

type FieldOption struct {
	ID    int64  `json:"id"`
	Label string `json:"label"`
	Color string `json:"color"`
}

type Field struct {
	FieldName     string        `json:"field_name"`
	FieldCode     string        `json:"field_code"`
	FieldType     string        `json:"field_type"`
	Options       []FieldOption `json:"options"`
	IsCustomField bool          `json:"is_custom_field"`
}

type Filter struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	ActiveFlag bool   `json:"active_flag"`
	UpdateTime string `json:"update_time"`
}

type Webhook struct {
	ID              int64  `json:"id"`
	SubscriptionURL string `json:"subscription_url"`
	EventAction     string `json:"event_action"`
	EventObject     string `json:"event_object"`
	IsActive        any    `json:"is_active"`
}

func id(v int64) string { return strconv.FormatInt(v, 10) }

func idPath(prefix string, v int64) string { return prefix + "/" + id(v) }

// ---------- account ----------

func (c *Client) Me(ctx context.Context) (*Me, error) {
	var out Me
	_, err := c.call(ctx, http.MethodGet, "/v1/users/me", nil, nil, &out)
	return &out, err
}

func (c *Client) Users(ctx context.Context) ([]User, error) {
	var out []User
	_, err := c.call(ctx, http.MethodGet, "/v1/users", nil, nil, &out)
	return out, err
}

// ---------- pipelines ----------

func (c *Client) Pipelines(ctx context.Context) ([]Pipeline, error) {
	return listV2[Pipeline](ctx, c, "/api/v2/pipelines", nil, 0)
}

func (c *Client) Stages(ctx context.Context) ([]Stage, error) {
	return listV2[Stage](ctx, c, "/api/v2/stages", nil, 0)
}

// ---------- deals ----------

func (c *Client) Deals(ctx context.Context, q url.Values, max int) ([]Deal, error) {
	return listV2[Deal](ctx, c, "/api/v2/deals", q, max)
}

func (c *Client) GetDeal(ctx context.Context, dealID int64) (*Deal, error) {
	var out Deal
	_, err := c.call(ctx, http.MethodGet, idPath("/api/v2/deals", dealID), nil, nil, &out)
	return &out, err
}

func (c *Client) CreateDeal(ctx context.Context, body map[string]any) (*Deal, error) {
	var out Deal
	_, err := c.call(ctx, http.MethodPost, "/api/v2/deals", nil, body, &out)
	return &out, err
}

func (c *Client) UpdateDeal(ctx context.Context, dealID int64, body map[string]any) (*Deal, error) {
	var out Deal
	_, err := c.call(ctx, http.MethodPatch, idPath("/api/v2/deals", dealID), nil, body, &out)
	return &out, err
}

func (c *Client) DeleteDeal(ctx context.Context, dealID int64) error {
	_, err := c.call(ctx, http.MethodDelete, idPath("/api/v2/deals", dealID), nil, nil, nil)
	return err
}

// ---------- persons ----------

func (c *Client) Persons(ctx context.Context, q url.Values, max int) ([]Person, error) {
	return listV2[Person](ctx, c, "/api/v2/persons", c.withMarketing(q), max)
}

// noMarketing remembers companies that refused the marketing_status field (no
// Campaigns add-on), so later reads stop asking for it.
var noMarketing sync.Map

func (c *Client) marketingOff() bool {
	_, ok := noMarketing.Load(c.company)
	return ok
}

func (c *Client) setMarketingOff() { noMarketing.Store(c.company, true) }

// withMarketing asks for the marketing status where the company has one.
func (c *Client) withMarketing(q url.Values) url.Values {
	if q == nil {
		q = url.Values{}
	}
	if !c.marketingOff() {
		q.Set("include_fields", "marketing_status")
	}
	return q
}

// GetPerson reads one person, with its marketing status where the company
// has one.
func (c *Client) GetPerson(ctx context.Context, personID int64) (*Person, error) {
	var out Person
	_, err := c.call(ctx, http.MethodGet, idPath("/api/v2/persons", personID), c.withMarketing(nil), nil, &out)
	return &out, err
}

// PersonsByIDs reads up to 100 persons at once.
func (c *Client) PersonsByIDs(ctx context.Context, ids []int64) ([]Person, error) {
	var out []Person
	for start := 0; start < len(ids); start += 100 {
		end := min(start+100, len(ids))
		parts := make([]string, 0, end-start)
		for _, v := range ids[start:end] {
			parts = append(parts, id(v))
		}
		page, err := c.Persons(ctx, url.Values{"ids": {strings.Join(parts, ",")}}, 0)
		if err != nil {
			return out, err
		}
		out = append(out, page...)
	}
	return out, nil
}

// FindPersonByEmail returns the id of the person with this address, or 0.
func (c *Client) FindPersonByEmail(ctx context.Context, email string) (int64, error) {
	var out struct {
		Items []struct {
			Item struct {
				ID int64 `json:"id"`
			} `json:"item"`
		} `json:"items"`
	}
	q := url.Values{"term": {email}, "fields": {"email"}, "exact_match": {"true"}, "limit": {"1"}}
	if _, err := c.call(ctx, http.MethodGet, "/api/v2/persons/search", q, nil, &out); err != nil {
		return 0, err
	}
	if len(out.Items) == 0 {
		return 0, nil
	}
	return out.Items[0].Item.ID, nil
}

func (c *Client) CreatePerson(ctx context.Context, body map[string]any) (*Person, error) {
	var out Person
	_, err := c.call(ctx, http.MethodPost, "/api/v2/persons", nil, body, &out)
	return &out, err
}

func (c *Client) UpdatePerson(ctx context.Context, personID int64, body map[string]any) (*Person, error) {
	var out Person
	_, err := c.call(ctx, http.MethodPatch, idPath("/api/v2/persons", personID), nil, body, &out)
	return &out, err
}

// ---------- organizations ----------

func (c *Client) GetOrganization(ctx context.Context, orgID int64) (*Organization, error) {
	var out Organization
	_, err := c.call(ctx, http.MethodGet, idPath("/api/v2/organizations", orgID), nil, nil, &out)
	return &out, err
}

// FindOrganization returns the organization named exactly name, or nil.
func (c *Client) FindOrganization(ctx context.Context, name string) (*Organization, error) {
	if len([]rune(strings.TrimSpace(name))) < 2 {
		return nil, nil
	}
	var out struct {
		Items []struct {
			Item Organization `json:"item"`
		} `json:"items"`
	}
	q := url.Values{"term": {name}, "fields": {"name"}, "exact_match": {"true"}, "limit": {"1"}}
	if _, err := c.call(ctx, http.MethodGet, "/api/v2/organizations/search", q, nil, &out); err != nil {
		return nil, err
	}
	if len(out.Items) == 0 {
		return nil, nil
	}
	o := out.Items[0].Item
	return &o, nil
}

func (c *Client) CreateOrganization(ctx context.Context, body map[string]any) (*Organization, error) {
	var out Organization
	_, err := c.call(ctx, http.MethodPost, "/api/v2/organizations", nil, body, &out)
	return &out, err
}

// ---------- activities ----------

func (c *Client) Activities(ctx context.Context, q url.Values, max int) ([]Activity, error) {
	return listV2[Activity](ctx, c, "/api/v2/activities", q, max)
}

func (c *Client) GetActivity(ctx context.Context, activityID int64) (*Activity, error) {
	var out Activity
	_, err := c.call(ctx, http.MethodGet, idPath("/api/v2/activities", activityID), nil, nil, &out)
	return &out, err
}

func (c *Client) CreateActivity(ctx context.Context, body map[string]any) (*Activity, error) {
	var out Activity
	_, err := c.call(ctx, http.MethodPost, "/api/v2/activities", nil, body, &out)
	return &out, err
}

func (c *Client) UpdateActivity(ctx context.Context, activityID int64, body map[string]any) (*Activity, error) {
	var out Activity
	_, err := c.call(ctx, http.MethodPatch, idPath("/api/v2/activities", activityID), nil, body, &out)
	return &out, err
}

func (c *Client) DeleteActivity(ctx context.Context, activityID int64) error {
	_, err := c.call(ctx, http.MethodDelete, idPath("/api/v2/activities", activityID), nil, nil, nil)
	return err
}

func (c *Client) ActivityTypes(ctx context.Context) ([]ActivityType, error) {
	var out []ActivityType
	_, err := c.call(ctx, http.MethodGet, "/v1/activityTypes", nil, nil, &out)
	return out, err
}

// ---------- notes ----------

func (c *Client) Notes(ctx context.Context, q url.Values, max int) ([]Note, error) {
	return listV1[Note](ctx, c, "/v1/notes", q, max)
}

func (c *Client) GetNote(ctx context.Context, noteID int64) (*Note, error) {
	var out Note
	_, err := c.call(ctx, http.MethodGet, idPath("/v1/notes", noteID), nil, nil, &out)
	return &out, err
}

func (c *Client) CreateNote(ctx context.Context, body map[string]any) (*Note, error) {
	var out Note
	_, err := c.call(ctx, http.MethodPost, "/v1/notes", nil, body, &out)
	return &out, err
}

func (c *Client) UpdateNote(ctx context.Context, noteID int64, content string) error {
	_, err := c.call(ctx, http.MethodPut, idPath("/v1/notes", noteID), nil, map[string]any{"content": content}, nil)
	return err
}

func (c *Client) DeleteNote(ctx context.Context, noteID int64) error {
	_, err := c.call(ctx, http.MethodDelete, idPath("/v1/notes", noteID), nil, nil, nil)
	return err
}

// ---------- fields ----------

func (c *Client) PersonFields(ctx context.Context) ([]Field, error) {
	return listV2[Field](ctx, c, "/api/v2/personFields", nil, 0)
}

func (c *Client) CreatePersonField(ctx context.Context, name, fieldType string, options []string) (*Field, error) {
	body := map[string]any{"field_name": name, "field_type": fieldType}
	if len(options) > 0 {
		opts := make([]map[string]string, 0, len(options))
		for _, o := range options {
			opts = append(opts, map[string]string{"label": o})
		}
		body["options"] = opts
	}
	var out Field
	_, err := c.call(ctx, http.MethodPost, "/api/v2/personFields", nil, body, &out)
	return &out, err
}

// ---------- filters ----------

// PersonFilters lists the company's saved people filters.
func (c *Client) PersonFilters(ctx context.Context) ([]Filter, error) {
	var out []Filter
	_, err := c.call(ctx, http.MethodGet, "/v1/filters", url.Values{"type": {"people"}}, nil, &out)
	return out, err
}

// ---------- leads ----------

func (c *Client) CreateLead(ctx context.Context, body map[string]any) error {
	_, err := c.call(ctx, http.MethodPost, "/v1/leads", nil, body, nil)
	return err
}

// ---------- webhooks ----------

func (c *Client) Webhooks(ctx context.Context) ([]Webhook, error) {
	var out []Webhook
	_, err := c.call(ctx, http.MethodGet, "/v1/webhooks", nil, nil, &out)
	return out, err
}

func (c *Client) CreateWebhook(ctx context.Context, body map[string]any) error {
	_, err := c.call(ctx, http.MethodPost, "/v1/webhooks", nil, body, nil)
	return err
}

func (c *Client) DeleteWebhook(ctx context.Context, webhookID int64) error {
	_, err := c.call(ctx, http.MethodDelete, idPath("/v1/webhooks", webhookID), nil, nil, nil)
	return err
}
