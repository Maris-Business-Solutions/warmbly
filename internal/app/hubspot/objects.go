package hubspot

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HubSpot-defined association type ids for the pairs Warmbly writes.
const (
	assocContactToCompany = 279
	assocDealToContact    = 3
	assocDealToCompany    = 341
	assocTaskToContact    = 204
	assocTaskToDeal       = 216
	assocNoteToContact    = 202
	assocNoteToDeal       = 214
	assocEmailToContact   = 198
	assocEmailToDeal      = 210
	assocEmailToCompany   = 186
	assocMeetingToContact = 200
	assocMeetingToDeal    = 212
)

// Object is any CRM record: contact, deal, task, note, email, meeting.
type Object struct {
	ID           string                     `json:"id"`
	Properties   map[string]*string         `json:"properties"`
	CreatedAt    time.Time                  `json:"createdAt"`
	UpdatedAt    time.Time                  `json:"updatedAt"`
	Archived     bool                       `json:"archived"`
	Associations map[string]associationPage `json:"associations,omitempty"`
}

type associationPage struct {
	Results []struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	} `json:"results"`
}

// Prop returns a property, "" when unset.
func (o Object) Prop(name string) string {
	if v, ok := o.Properties[name]; ok && v != nil {
		return *v
	}
	return ""
}

// Associated lists the ids associated under one object type.
func (o Object) Associated(objectType string) []string {
	page, ok := o.Associations[objectType]
	if !ok {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, r := range page.Results {
		if !seen[r.ID] {
			seen[r.ID] = true
			out = append(out, r.ID)
		}
	}
	return out
}

// Assoc is an association written with a create.
type Assoc struct {
	ToID   string
	TypeID int
}

func assocBody(assocs []Assoc) []map[string]any {
	out := make([]map[string]any, 0, len(assocs))
	for _, a := range assocs {
		if a.ToID == "" {
			continue
		}
		out = append(out, map[string]any{
			"to":    map[string]any{"id": a.ToID},
			"types": []map[string]any{{"associationCategory": "HUBSPOT_DEFINED", "associationTypeId": a.TypeID}},
		})
	}
	return out
}

// Owner is a HubSpot user who can own records.
type Owner struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Archived  bool   `json:"archived"`
}

// Owners lists active and archived owners.
func (c *Client) Owners(ctx context.Context) ([]Owner, error) {
	var all []Owner
	for _, archived := range []bool{false, true} {
		after := ""
		for page := 0; page < 50; page++ {
			q := url.Values{"limit": {"100"}, "archived": {fmt.Sprint(archived)}}
			if after != "" {
				q.Set("after", after)
			}
			var out struct {
				Results []Owner `json:"results"`
				Paging  paging  `json:"paging"`
			}
			if err := c.do(ctx, http.MethodGet, "/crm/v3/owners/?"+q.Encode(), nil, &out); err != nil {
				return nil, err
			}
			for i := range out.Results {
				out.Results[i].Archived = archived
			}
			all = append(all, out.Results...)
			if after = out.Paging.Next.After; after == "" {
				break
			}
		}
	}
	return all, nil
}

type paging struct {
	Next struct {
		After string `json:"after"`
	} `json:"next"`
}

// Pipeline is a deal pipeline with its stages.
type Pipeline struct {
	ID           string  `json:"id"`
	Label        string  `json:"label"`
	DisplayOrder int     `json:"displayOrder"`
	Archived     bool    `json:"archived"`
	Stages       []Stage `json:"stages"`
}

// Stage is one pipeline stage. Metadata carries isClosed and probability as strings.
type Stage struct {
	ID           string            `json:"id"`
	Label        string            `json:"label"`
	DisplayOrder int               `json:"displayOrder"`
	Archived     bool              `json:"archived"`
	Metadata     map[string]string `json:"metadata"`
}

// DealPipelines lists the portal's deal pipelines.
func (c *Client) DealPipelines(ctx context.Context) ([]Pipeline, error) {
	var out struct {
		Results []Pipeline `json:"results"`
	}
	if err := c.do(ctx, http.MethodGet, "/crm/v3/pipelines/deals", nil, &out); err != nil {
		return nil, err
	}
	return out.Results, nil
}

// Property is a CRM property definition.
type Property struct {
	Name                 string           `json:"name"`
	Label                string           `json:"label"`
	Type                 string           `json:"type"`
	FieldType            string           `json:"fieldType"`
	GroupName            string           `json:"groupName"`
	Options              []PropertyOption `json:"options"`
	Hidden               bool             `json:"hidden"`
	Calculated           bool             `json:"calculated"`
	ModificationMetadata struct {
		ReadOnlyValue bool `json:"readOnlyValue"`
	} `json:"modificationMetadata"`
}

// PropertyOption is one enumeration value.
type PropertyOption struct {
	Label        string `json:"label"`
	Value        string `json:"value"`
	DisplayOrder int    `json:"displayOrder"`
	Hidden       bool   `json:"hidden"`
}

// Properties lists an object type's properties.
func (c *Client) Properties(ctx context.Context, objectType string) ([]Property, error) {
	var out struct {
		Results []Property `json:"results"`
	}
	if err := c.do(ctx, http.MethodGet, "/crm/v3/properties/"+objectType, nil, &out); err != nil {
		return nil, err
	}
	return out.Results, nil
}

// PropertyDef is a property Warmbly creates.
type PropertyDef struct {
	Name        string           `json:"name"`
	Label       string           `json:"label"`
	Type        string           `json:"type"`
	FieldType   string           `json:"fieldType"`
	GroupName   string           `json:"groupName"`
	Description string           `json:"description,omitempty"`
	Options     []PropertyOption `json:"options,omitempty"`
}

// EnsureContactProperties creates the property group and any missing property.
// Existing ones are left as the portal's admins may have tuned them.
func (c *Client) EnsureContactProperties(ctx context.Context, group, groupLabel string, defs []PropertyDef) error {
	err := c.do(ctx, http.MethodPost, "/crm/v3/properties/contacts/groups",
		map[string]any{"name": group, "label": groupLabel, "displayOrder": -1}, nil)
	if ae, ok := AsAPIError(err); err != nil && !(ok && ae.Conflict()) {
		return err
	}
	existing, err := c.Properties(ctx, "contacts")
	if err != nil {
		return err
	}
	have := map[string]bool{}
	for _, p := range existing {
		have[p.Name] = true
	}
	var missing []PropertyDef
	for _, d := range defs {
		if !have[d.Name] {
			missing = append(missing, d)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	err = c.do(ctx, http.MethodPost, "/crm/v3/properties/contacts/batch/create", map[string]any{"inputs": missing}, nil)
	if ae, ok := AsAPIError(err); err != nil && !(ok && ae.Conflict()) {
		return err
	}
	return nil
}

// ContactUpsert is one contact written by email.
type ContactUpsert struct {
	Email      string
	Properties map[string]string
}

// UpsertContacts creates or updates contacts keyed by email, at most 100 per call.
// The result maps lowercased email to record id.
func (c *Client) UpsertContacts(ctx context.Context, in []ContactUpsert) (map[string]string, error) {
	out := map[string]string{}
	for start := 0; start < len(in); start += 100 {
		end := min(start+100, len(in))
		inputs := make([]map[string]any, 0, end-start)
		for _, ci := range in[start:end] {
			props := map[string]string{}
			for k, v := range ci.Properties {
				props[k] = v
			}
			props["email"] = ci.Email
			inputs = append(inputs, map[string]any{"idProperty": "email", "id": ci.Email, "properties": props})
		}
		var resp struct {
			Results []Object `json:"results"`
		}
		if err := c.do(ctx, http.MethodPost, "/crm/v3/objects/contacts/batch/upsert", map[string]any{"inputs": inputs}, &resp); err != nil {
			return out, err
		}
		for _, r := range resp.Results {
			out[strings.ToLower(r.Prop("email"))] = r.ID
		}
	}
	return out, nil
}

// GetObject reads one record with properties and associations.
func (c *Client) GetObject(ctx context.Context, objectType, id string, props, assocs []string) (*Object, error) {
	q := url.Values{}
	if len(props) > 0 {
		q.Set("properties", strings.Join(props, ","))
	}
	if len(assocs) > 0 {
		q.Set("associations", strings.Join(assocs, ","))
	}
	var out Object
	if err := c.do(ctx, http.MethodGet, "/crm/v3/objects/"+objectType+"/"+url.PathEscape(id)+"?"+q.Encode(), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ReadObjects reads records by id, 100 per call.
func (c *Client) ReadObjects(ctx context.Context, objectType string, ids, props []string) ([]Object, error) {
	var all []Object
	for start := 0; start < len(ids); start += 100 {
		end := min(start+100, len(ids))
		inputs := make([]map[string]string, 0, end-start)
		for _, id := range ids[start:end] {
			inputs = append(inputs, map[string]string{"id": id})
		}
		var out struct {
			Results []Object `json:"results"`
		}
		if err := c.do(ctx, http.MethodPost, "/crm/v3/objects/"+objectType+"/batch/read",
			map[string]any{"inputs": inputs, "properties": props}, &out); err != nil {
			return all, err
		}
		all = append(all, out.Results...)
	}
	return all, nil
}

// Filter is one search condition.
type Filter struct {
	PropertyName string   `json:"propertyName"`
	Operator     string   `json:"operator"`
	Value        string   `json:"value,omitempty"`
	Values       []string `json:"values,omitempty"`
}

// SearchPage is one page of search results.
type SearchPage struct {
	Total   int      `json:"total"`
	Results []Object `json:"results"`
	Paging  paging   `json:"paging"`
}

// Search runs a CRM search: AND inside a group, OR across groups.
func (c *Client) Search(ctx context.Context, objectType string, groups [][]Filter, sortProp string, props []string, after string, limit int) (*SearchPage, error) {
	fg := make([]map[string]any, 0, len(groups))
	for _, g := range groups {
		fg = append(fg, map[string]any{"filters": g})
	}
	body := map[string]any{"filterGroups": fg, "properties": props, "limit": limit}
	if sortProp != "" {
		body["sorts"] = []map[string]string{{"propertyName": sortProp, "direction": "ASCENDING"}}
	}
	if after != "" {
		body["after"] = after
	}
	var out SearchPage
	if err := c.do(ctx, http.MethodPost, "/crm/v3/objects/"+objectType+"/search", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Create writes a record with its associations.
func (c *Client) Create(ctx context.Context, objectType string, props map[string]string, assocs []Assoc) (*Object, error) {
	body := map[string]any{"properties": props}
	if a := assocBody(assocs); len(a) > 0 {
		body["associations"] = a
	}
	var out Object
	if err := c.do(ctx, http.MethodPost, "/crm/v3/objects/"+objectType, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update patches a record's properties.
func (c *Client) Update(ctx context.Context, objectType, id string, props map[string]string) (*Object, error) {
	var out Object
	if err := c.do(ctx, http.MethodPatch, "/crm/v3/objects/"+objectType+"/"+url.PathEscape(id),
		map[string]any{"properties": props}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Archive deletes a record (HubSpot keeps it restorable for 90 days).
func (c *Client) Archive(ctx context.Context, objectType, id string) error {
	err := c.do(ctx, http.MethodDelete, "/crm/v3/objects/"+objectType+"/"+url.PathEscape(id), nil, nil)
	if ae, ok := AsAPIError(err); ok && ae.NotFound() {
		return nil
	}
	return err
}

// ObjectUpdate is one row of a batch update.
type ObjectUpdate struct {
	ID         string
	Properties map[string]string
}

// BatchUpdate patches up to 100 records per call.
func (c *Client) BatchUpdate(ctx context.Context, objectType string, in []ObjectUpdate) error {
	for start := 0; start < len(in); start += 100 {
		end := min(start+100, len(in))
		inputs := make([]map[string]any, 0, end-start)
		for _, u := range in[start:end] {
			inputs = append(inputs, map[string]any{"id": u.ID, "properties": u.Properties})
		}
		if err := c.do(ctx, http.MethodPost, "/crm/v3/objects/"+objectType+"/batch/update", map[string]any{"inputs": inputs}, nil); err != nil {
			return err
		}
	}
	return nil
}

// BatchArchive deletes up to 100 records per call.
func (c *Client) BatchArchive(ctx context.Context, objectType string, ids []string) error {
	for start := 0; start < len(ids); start += 100 {
		end := min(start+100, len(ids))
		inputs := make([]map[string]string, 0, end-start)
		for _, id := range ids[start:end] {
			inputs = append(inputs, map[string]string{"id": id})
		}
		if err := c.do(ctx, http.MethodPost, "/crm/v3/objects/"+objectType+"/batch/archive", map[string]any{"inputs": inputs}, nil); err != nil {
			return err
		}
	}
	return nil
}

// AssociateDefault links two records with HubSpot's default association label.
func (c *Client) AssociateDefault(ctx context.Context, fromType, fromID, toType, toID string) error {
	return c.do(ctx, http.MethodPut, fmt.Sprintf("/crm/v4/objects/%s/%s/associations/default/%s/%s",
		fromType, url.PathEscape(fromID), toType, url.PathEscape(toID)), nil, nil)
}

// AssociatedIDs lists the ids of toType records associated with one record.
func (c *Client) AssociatedIDs(ctx context.Context, fromType, fromID, toType string, max int) ([]string, error) {
	var ids []string
	after := ""
	for len(ids) < max {
		q := url.Values{"limit": {"500"}}
		if after != "" {
			q.Set("after", after)
		}
		var out struct {
			Results []struct {
				ToObjectID any `json:"toObjectId"`
			} `json:"results"`
			Paging paging `json:"paging"`
		}
		if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/crm/v4/objects/%s/%s/associations/%s?%s",
			fromType, url.PathEscape(fromID), toType, q.Encode()), nil, &out); err != nil {
			return ids, err
		}
		for _, r := range out.Results {
			ids = append(ids, fmt.Sprint(r.ToObjectID))
		}
		if after = out.Paging.Next.After; after == "" {
			break
		}
	}
	if len(ids) > max {
		ids = ids[:max]
	}
	return ids, nil
}

// BatchAssociations maps each from id to its associated toType ids.
func (c *Client) BatchAssociations(ctx context.Context, fromType, toType string, ids []string) (map[string][]string, error) {
	out := map[string][]string{}
	for start := 0; start < len(ids); start += 100 {
		end := min(start+100, len(ids))
		inputs := make([]map[string]string, 0, end-start)
		for _, id := range ids[start:end] {
			inputs = append(inputs, map[string]string{"id": id})
		}
		var resp struct {
			Results []struct {
				From struct {
					ID string `json:"id"`
				} `json:"from"`
				To []struct {
					ToObjectID any `json:"toObjectId"`
				} `json:"to"`
			} `json:"results"`
		}
		if err := c.do(ctx, http.MethodPost, fmt.Sprintf("/crm/v4/associations/%s/%s/batch/read", fromType, toType),
			map[string]any{"inputs": inputs}, &resp); err != nil {
			return out, err
		}
		for _, r := range resp.Results {
			for _, t := range r.To {
				out[r.From.ID] = append(out[r.From.ID], fmt.Sprint(t.ToObjectID))
			}
		}
	}
	return out, nil
}

// List is a HubSpot contact list (segment).
type List struct {
	ListID         string `json:"listId"`
	Name           string `json:"name"`
	ProcessingType string `json:"processingType"`
	ObjectTypeID   string `json:"objectTypeId"`
	UpdatedAt      string `json:"updatedAt"`
	Additional     struct {
		Size string `json:"hs_list_size"`
	} `json:"additionalProperties"`
}

// SearchLists finds contact lists by name.
func (c *Client) SearchLists(ctx context.Context, query string, offset, count int) ([]List, bool, int, error) {
	body := map[string]any{
		"query":                   query,
		"offset":                  offset,
		"count":                   count,
		"additionalProperties":    []string{"hs_list_size"},
		"processingTypes":         []string{"MANUAL", "DYNAMIC", "SNAPSHOT"},
		"sort":                    "HS_UPDATED_AT",
		"listIds":                 []string{},
		"objectTypeId":            "0-1",
		"includeFilters":          false,
		"includeAdditionalFields": true,
	}
	var out struct {
		Lists   []List `json:"lists"`
		HasMore bool   `json:"hasMore"`
		Offset  int    `json:"offset"`
	}
	if err := c.do(ctx, http.MethodPost, "/crm/v3/lists/search", body, &out); err != nil {
		return nil, false, 0, err
	}
	var contacts []List
	for _, l := range out.Lists {
		if l.ObjectTypeID == "" || l.ObjectTypeID == "0-1" {
			contacts = append(contacts, l)
		}
	}
	return contacts, out.HasMore, out.Offset, nil
}

// GetList reads one list.
func (c *Client) GetList(ctx context.Context, listID string) (*List, error) {
	var out struct {
		List List `json:"list"`
	}
	if err := c.do(ctx, http.MethodGet, "/crm/v3/lists/"+url.PathEscape(listID)+"?includeFilters=false", nil, &out); err != nil {
		return nil, err
	}
	return &out.List, nil
}

// ListMemberIDs returns up to max record ids in a list and whether it had more.
func (c *Client) ListMemberIDs(ctx context.Context, listID string, max int) ([]string, bool, error) {
	var ids []string
	after := ""
	for {
		q := url.Values{"limit": {"250"}}
		if after != "" {
			q.Set("after", after)
		}
		var out struct {
			Results []struct {
				RecordID string `json:"recordId"`
			} `json:"results"`
			Paging paging `json:"paging"`
		}
		if err := c.do(ctx, http.MethodGet, "/crm/v3/lists/"+url.PathEscape(listID)+"/memberships?"+q.Encode(), nil, &out); err != nil {
			return ids, false, err
		}
		for i, r := range out.Results {
			ids = append(ids, r.RecordID)
			if len(ids) >= max {
				return ids, i < len(out.Results)-1 || out.Paging.Next.After != "", nil
			}
		}
		if after = out.Paging.Next.After; after == "" {
			return ids, false, nil
		}
	}
}

// FindCompanyByDomain returns the first company on a domain, or nil.
func (c *Client) FindCompanyByDomain(ctx context.Context, domain string) (*Object, error) {
	page, err := c.Search(ctx, "companies", [][]Filter{{{PropertyName: "domain", Operator: "EQ", Value: domain}}},
		"", []string{"name", "domain"}, "", 1)
	if err != nil {
		return nil, err
	}
	if len(page.Results) == 0 {
		return nil, nil
	}
	return &page.Results[0], nil
}
