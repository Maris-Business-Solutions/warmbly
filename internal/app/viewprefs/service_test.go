package viewprefs

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/warmbly/warmbly/internal/models"
)

func cols(v ...string) *[]string { return &v }

func TestValidateNormalizesAndRejects(t *testing.T) {
	ok := models.ViewPreferencesUpdate{
		Columns: cols("name", "company", "custom:Company   Mobile", "phone"),
		Sort:    &models.ViewSort{By: " custom:Industry ", Reverse: true},
	}
	if xerr := validate(models.ViewContacts, &ok); xerr != nil {
		t.Fatalf("valid layout rejected: %v", xerr)
	}
	if (*ok.Columns)[2] != "custom:Company Mobile" {
		t.Fatalf("custom column not normalized: %q", (*ok.Columns)[2])
	}
	if ok.Sort.By != "custom:Industry" {
		t.Fatalf("sort not normalized: %q", ok.Sort.By)
	}

	blank := models.ViewPreferencesUpdate{Sort: &models.ViewSort{By: "  ", Reverse: true}}
	if xerr := validate(models.ViewContacts, &blank); xerr != nil || blank.Sort == nil || blank.Sort.By != "" || blank.Sort.Reverse {
		t.Fatalf("a blank sort means the default: %v %+v", xerr, blank.Sort)
	}

	partial := models.ViewPreferencesUpdate{}
	if xerr := validate(models.ViewContacts, &partial); xerr != nil {
		t.Fatalf("an empty update keeps everything: %v", xerr)
	}

	leads := models.ViewPreferencesUpdate{Columns: cols("progress", "opened", "sender")}
	if xerr := validate(models.ViewCampaignLeads, &leads); xerr != nil {
		t.Fatalf("leads columns rejected: %v", xerr)
	}

	for name, tc := range map[string]struct {
		view string
		upd  models.ViewPreferencesUpdate
		code string
	}{
		"duplicate":           {models.ViewContacts, models.ViewPreferencesUpdate{Columns: cols("company", "custom:A", "custom:A")}, "duplicate_column"},
		"unknown builtin":     {models.ViewContacts, models.ViewPreferencesUpdate{Columns: cols("nonexistent_col")}, "invalid_column"},
		"other view's column": {models.ViewContacts, models.ViewPreferencesUpdate{Columns: cols("opened")}, "invalid_column"},
		"bad case":            {models.ViewContacts, models.ViewPreferencesUpdate{Columns: cols("Company")}, "invalid_column"},
		"bad custom key":      {models.ViewContacts, models.ViewPreferencesUpdate{Columns: cols("custom:Revenue ($)")}, "invalid_column"},
		"too long":            {models.ViewContacts, models.ViewPreferencesUpdate{Columns: cols("custom:" + strings.Repeat("a", 300))}, "invalid_column"},
		"bad sort":            {models.ViewContacts, models.ViewPreferencesUpdate{Sort: &models.ViewSort{By: "drop table"}}, "invalid_sort"},
		"column as sort":      {models.ViewContacts, models.ViewPreferencesUpdate{Sort: &models.ViewSort{By: "status"}}, "invalid_sort"},
		"too many":            {models.ViewContacts, models.ViewPreferencesUpdate{Columns: manyColumns(models.ViewPreferencesMaxColumns + 1)}, "too_many_columns"},
	} {
		xerr := validate(tc.view, &tc.upd)
		if xerr == nil {
			t.Errorf("%s: accepted", name)
			continue
		}
		if xerr.Identifier != tc.code {
			t.Errorf("%s: code %q, want %q", name, xerr.Identifier, tc.code)
		}
	}
}

func manyColumns(n int) *[]string {
	out := make([]string, n)
	for i := range out {
		out[i] = "custom:f" + strings.Repeat("x", i%5) + string(rune('a'+i%26)) + strings.Repeat("y", i/26)
	}
	return &out
}

func TestValidateUniboxRailLayout(t *testing.T) {
	upd := models.ViewPreferencesUpdate{Layout: []byte(`{
		"favorites": [{"key": "folder:inbox", "name": "  Inbox   work "}, {"key": "folder:inbox"}],
		"hidden": ["view:today", "view:today"],
		"order": {"mail": ["folder:sent", "folder:inbox"]},
		"section_order": ["favorites", "mail"]
	}`)}
	if xerr := validate(models.ViewUniboxRail, &upd); xerr != nil {
		t.Fatalf("valid layout rejected: %v", xerr)
	}
	var got models.UniboxRailLayout
	if err := json.Unmarshal(upd.Layout, &got); err != nil {
		t.Fatalf("stored layout does not decode: %v", err)
	}
	if len(got.Favorites) != 1 || got.Favorites[0].Name != "Inbox work" {
		t.Fatalf("favorites not normalized: %+v", got.Favorites)
	}
	if len(got.Hidden) != 1 {
		t.Fatalf("hidden not deduplicated: %v", got.Hidden)
	}

	null := models.ViewPreferencesUpdate{Layout: []byte(`null`)}
	if xerr := validate(models.ViewUniboxRail, &null); xerr != nil || string(null.Layout) == "null" {
		t.Fatalf("null is the default layout: %v %s", xerr, null.Layout)
	}

	for name, tc := range map[string]struct {
		view string
		upd  models.ViewPreferencesUpdate
		code string
	}{
		"layout on a column view": {models.ViewContacts, models.ViewPreferencesUpdate{Layout: []byte(`{}`)}, "invalid_layout"},
		"unknown field":           {models.ViewUniboxRail, models.ViewPreferencesUpdate{Layout: []byte(`{"widths": 3}`)}, "invalid_layout"},
		"not an object":           {models.ViewUniboxRail, models.ViewPreferencesUpdate{Layout: []byte(`[]`)}, "invalid_layout"},
		"empty key":               {models.ViewUniboxRail, models.ViewPreferencesUpdate{Layout: []byte(`{"hidden": [""]}`)}, "invalid_layout"},
		"long key":                {models.ViewUniboxRail, models.ViewPreferencesUpdate{Layout: []byte(`{"hidden": ["` + strings.Repeat("a", models.UniboxRailKeyMaxLength+1) + `"]}`)}, "invalid_layout"},
		"control in key":          {models.ViewUniboxRail, models.ViewPreferencesUpdate{Layout: []byte(`{"hidden": ["a\u0007b"]}`)}, "invalid_layout"},
		"long name":               {models.ViewUniboxRail, models.ViewPreferencesUpdate{Layout: []byte(`{"favorites": [{"key": "k", "name": "` + strings.Repeat("n", models.UniboxRailFavoriteNameMax+1) + `"}]}`)}, "invalid_layout"},
		"columns on the rail":     {models.ViewUniboxRail, models.ViewPreferencesUpdate{Columns: cols("name")}, "invalid_column"},
		"sort on the rail":        {models.ViewUniboxRail, models.ViewPreferencesUpdate{Sort: &models.ViewSort{By: "created_at"}}, "invalid_sort"},
	} {
		xerr := validate(tc.view, &tc.upd)
		if xerr == nil {
			t.Errorf("%s: accepted", name)
			continue
		}
		if xerr.Identifier != tc.code {
			t.Errorf("%s: code %q, want %q", name, xerr.Identifier, tc.code)
		}
	}
}
