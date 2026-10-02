package models

import (
	"encoding/json"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Dashboard lists whose layout a member can save. A new list gets a constant
// here before the dashboard can persist a layout for it.
const (
	ViewContacts      = "contacts"
	ViewCampaignLeads = "campaign_leads"
	// ViewUniboxRail is the unibox scope rail. It has no columns or sort; its
	// layout is a UniboxRailLayout.
	ViewUniboxRail = "unibox_rail"
)

// KnownViews is the set of view names GET/PUT /me/views/:view accepts.
var KnownViews = map[string]bool{
	ViewContacts:      true,
	ViewCampaignLeads: true,
	ViewUniboxRail:    true,
}

// ViewHasLayout reports whether a view saves a layout document rather than
// columns and a sort.
func ViewHasLayout(view string) bool {
	return view == ViewUniboxRail
}

// ViewBuiltinColumns lists the built-in column ids each view can render, in
// step with the dashboard's column registry (web/src/components/app/contacts/
// columns.tsx). A saved layout may name only these and custom fields.
var ViewBuiltinColumns = map[string][]string{
	ViewContacts:      {"name", "company", "phone", "mail_host", "status", "campaigns", "created_at", "updated_at"},
	ViewCampaignLeads: {"name", "company", "phone", "mail_host", "progress", "opened", "clicked", "replied", "current_step", "sender", "last_activity", "created_at", "updated_at"},
}

// ContactBuiltinSorts are the sort_by values the contacts search knows besides
// "custom:<key>", the same set contactSorts holds in the repository.
var ContactBuiltinSorts = map[string]bool{
	"created_at": true, "updated_at": true, "first_name": true, "last_name": true,
	"email": true, "company": true, "phone": true, "campaign_count": true, "mail_host": true,
}

// ViewPreferencesMaxColumns bounds one layout; no list here has anywhere near
// this many columns to show.
const ViewPreferencesMaxColumns = 64

// ViewColumnIDMaxLength fits "custom:" plus the longest custom-field key.
const ViewColumnIDMaxLength = 300

// ViewColumnCustomPrefix marks a column that shows a contact custom field, the
// same addressing the export uses ("custom:Industry").
const ViewColumnCustomPrefix = "custom:"

// ViewSort is the saved ordering of a list, in the contacts search's terms.
type ViewSort struct {
	By      string `json:"by"`
	Reverse bool   `json:"reverse"`
}

// ViewPreferences is one member's saved layout for one list in one workspace.
// An empty Columns means "the default layout"; a nil Sort means "the default
// sort".
type ViewPreferences struct {
	View    string    `json:"view"`
	Columns []string  `json:"columns"`
	Sort    *ViewSort `json:"sort,omitempty"`
	// Layout is the saved document of a view ViewHasLayout names, already
	// validated on write; absent when none is saved.
	Layout    json.RawMessage `json:"layout,omitempty"`
	UpdatedAt *time.Time      `json:"updated_at,omitempty"`
}

// ViewPreferencesUpdate is a partial write: a nil field keeps what is saved,
// so a sort click made before the layout has loaded cannot erase the columns.
type ViewPreferencesUpdate struct {
	Columns *[]string
	Sort    *ViewSort
	Layout  json.RawMessage
}

// UniboxRailLayout is how one member arranges the unibox scope rail in one
// workspace. Every key is a rail scope key ("folder:inbox", "mailbox:<id>");
// the rail ignores keys it no longer shows, so none is checked against the
// workspace. Section folds and pane widths are per device and not part of it.
type UniboxRailLayout struct {
	Favorites    []UniboxRailFavorite `json:"favorites"`
	Hidden       []string             `json:"hidden"`
	Order        map[string][]string  `json:"order"`
	SectionOrder []string             `json:"section_order"`
}

// UniboxRailFavorite is a scope pinned to the rail's Favorites section, with
// an optional name the member gave it.
type UniboxRailFavorite struct {
	Key  string `json:"key"`
	Name string `json:"name,omitempty"`
}

// Bounds on a rail layout, far above what a rail holds.
const (
	UniboxRailMaxKeys         = 500
	UniboxRailMaxSections     = 32
	UniboxRailKeyMaxLength    = 200
	UniboxRailFavoriteNameMax = 40
)

// Normalize validates the layout and returns it with duplicate keys dropped,
// favorite names trimmed and nil lists made empty, or false when anything is
// out of bounds.
func (l UniboxRailLayout) Normalize() (UniboxRailLayout, bool) {
	out := UniboxRailLayout{Order: map[string][]string{}}
	var ok bool
	if out.Hidden, ok = railKeys(l.Hidden, UniboxRailMaxKeys); !ok {
		return out, false
	}
	if out.SectionOrder, ok = railKeys(l.SectionOrder, UniboxRailMaxSections); !ok {
		return out, false
	}
	if len(l.Order) > UniboxRailMaxSections {
		return out, false
	}
	for section, keys := range l.Order {
		if !validRailKey(section) {
			return out, false
		}
		if out.Order[section], ok = railKeys(keys, UniboxRailMaxKeys); !ok {
			return out, false
		}
	}
	if len(l.Favorites) > UniboxRailMaxKeys {
		return out, false
	}
	out.Favorites = make([]UniboxRailFavorite, 0, len(l.Favorites))
	seen := make(map[string]bool, len(l.Favorites))
	for _, f := range l.Favorites {
		if !validRailKey(f.Key) {
			return out, false
		}
		name := strings.Join(strings.Fields(f.Name), " ")
		if utf8.RuneCountInString(name) > UniboxRailFavoriteNameMax || strings.IndexFunc(name, unicode.IsControl) >= 0 {
			return out, false
		}
		if seen[f.Key] {
			continue
		}
		seen[f.Key] = true
		out.Favorites = append(out.Favorites, UniboxRailFavorite{Key: f.Key, Name: name})
	}
	return out, true
}

func railKeys(keys []string, limit int) ([]string, bool) {
	if len(keys) > limit {
		return nil, false
	}
	out := make([]string, 0, len(keys))
	seen := make(map[string]bool, len(keys))
	for _, k := range keys {
		if !validRailKey(k) {
			return nil, false
		}
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	return out, true
}

func validRailKey(k string) bool {
	return k != "" && len(k) <= UniboxRailKeyMaxLength && utf8.ValidString(k) &&
		strings.IndexFunc(k, unicode.IsControl) < 0
}

// ContactSortCustomPrefix is the sort_by form that orders the contacts list on
// a custom field: "custom:<key>". The key is normalized the way custom-field
// names are on write, so "custom:Company  Mobile" and "custom:Company Mobile"
// are the same sort.
const ContactSortCustomPrefix = "custom:"

// ContactSortCustomField returns the custom-field key a sort_by names, or
// false when sort_by is not the custom form. Whether the key is a valid
// custom-field name is the caller's check.
func ContactSortCustomField(sortBy string) (string, bool) {
	if !strings.HasPrefix(sortBy, ContactSortCustomPrefix) {
		return "", false
	}
	key := strings.Join(strings.Fields(strings.TrimPrefix(sortBy, ContactSortCustomPrefix)), " ")
	return key, true
}
