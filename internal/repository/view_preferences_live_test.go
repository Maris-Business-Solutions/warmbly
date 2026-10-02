package repository

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/models"
)

// A saved layout is written in parts: the chooser sends columns, a header
// click sends the sort, and neither may erase the other. The COALESCE in the
// upsert is what keeps them apart, and only a database can check it.
//
// Run against the dev stack:
//
//	WARMBLY_TEST_DB=postgres://warmbly:warmbly@localhost:15432/warmbly_dev?sslmode=disable \
//	  go test ./internal/repository/ -run LiveViewPreferences -v
func TestLiveViewPreferencesPartialWrites(t *testing.T) {
	handle, pool := liveContactDB(t)
	ctx := context.Background()
	user, org := uuid.New(), uuid.New()
	tag := org.String()[:8]
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("fixture %q: %v", sql[:min(60, len(sql))], err)
		}
	}
	exec(`INSERT INTO users (id, first_name, last_name, email, password_hash) VALUES ($1, 'View', 'Live', $2, 'x')`, user, "i605v-"+tag+"@test.local")
	exec(`INSERT INTO organizations (id, name, slug, owner_user_id) VALUES ($1, 'Issue 605 views', $2, $3)`, org, "i605v-"+tag, user)
	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM organizations WHERE id = $1`, org)
		_, _ = pool.Exec(c, `DELETE FROM users WHERE id = $1`, user)
	})
	repo := NewViewPreferencesRepository(handle.Pool)

	none, err := repo.Get(ctx, user, org, models.ViewContacts)
	if err != nil || none != nil {
		t.Fatalf("nothing saved yet: %v %+v", err, none)
	}

	// A sort click before any layout exists creates the row with the default
	// columns, not with nothing.
	got, err := repo.Upsert(ctx, user, org, models.ViewContacts, models.ViewPreferencesUpdate{Sort: &models.ViewSort{By: "company", Reverse: true}})
	if err != nil {
		t.Fatalf("sort-only insert: %v", err)
	}
	if len(got.Columns) != 0 || got.Sort == nil || got.Sort.By != "company" || !got.Sort.Reverse {
		t.Fatalf("sort-only insert saved %+v", got)
	}

	cols := []string{"name", "custom:Industry", "company"}
	got, err = repo.Upsert(ctx, user, org, models.ViewContacts, models.ViewPreferencesUpdate{Columns: &cols})
	if err != nil {
		t.Fatalf("columns-only update: %v", err)
	}
	if got.Sort == nil || got.Sort.By != "company" {
		t.Fatalf("a columns-only write erased the sort: %+v", got)
	}
	if len(got.Columns) != 3 || got.Columns[1] != "custom:Industry" {
		t.Fatalf("columns not saved: %+v", got.Columns)
	}

	got, err = repo.Upsert(ctx, user, org, models.ViewContacts, models.ViewPreferencesUpdate{Sort: &models.ViewSort{By: "custom:Industry"}})
	if err != nil {
		t.Fatalf("sort-only update: %v", err)
	}
	if len(got.Columns) != 3 {
		t.Fatalf("a sort-only write erased the columns: %+v", got)
	}
	if got.Sort == nil || got.Sort.By != "custom:Industry" || got.Sort.Reverse {
		t.Fatalf("sort not updated: %+v", got.Sort)
	}

	// The blank sort is the default sort, read back as none.
	got, err = repo.Upsert(ctx, user, org, models.ViewContacts, models.ViewPreferencesUpdate{Sort: &models.ViewSort{}})
	if err != nil {
		t.Fatalf("sort reset: %v", err)
	}
	if got.Sort != nil || len(got.Columns) != 3 {
		t.Fatalf("sort reset saved %+v", got)
	}

	// Each view is its own row, and Get sees what Upsert returned.
	if other, err := repo.Get(ctx, user, org, models.ViewCampaignLeads); err != nil || other != nil {
		t.Fatalf("leads view leaked: %v %+v", err, other)
	}
	again, err := repo.Get(ctx, user, org, models.ViewContacts)
	if err != nil || again == nil || len(again.Columns) != 3 || again.Sort != nil {
		t.Fatalf("Get after writes: %v %+v", err, again)
	}

	if err := repo.Delete(ctx, user, org, models.ViewContacts); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if gone, err := repo.Get(ctx, user, org, models.ViewContacts); err != nil || gone != nil {
		t.Fatalf("delete left a row: %v %+v", err, gone)
	}
}

// The rail's layout is a third part of the same row: writing it keeps the
// rest, and a later write without it keeps the layout.
func TestLiveViewPreferencesUniboxRailLayout(t *testing.T) {
	handle, pool := liveContactDB(t)
	ctx := context.Background()
	user, org := uuid.New(), uuid.New()
	tag := org.String()[:8]
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("fixture %q: %v", sql[:min(60, len(sql))], err)
		}
	}
	exec(`INSERT INTO users (id, first_name, last_name, email, password_hash) VALUES ($1, 'Rail', 'Live', $2, 'x')`, user, "i779-"+tag+"@test.local")
	exec(`INSERT INTO organizations (id, name, slug, owner_user_id) VALUES ($1, 'Issue 779 rail', $2, $3)`, org, "i779-"+tag, user)
	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM organizations WHERE id = $1`, org)
		_, _ = pool.Exec(c, `DELETE FROM users WHERE id = $1`, user)
	})
	repo := NewViewPreferencesRepository(handle.Pool)

	layout := []byte(`{"favorites":[{"key":"folder:inbox","name":"Inbox work"}],"hidden":["view:today"],"order":{"mail":["folder:sent","folder:inbox"]},"section_order":["favorites","mail"]}`)
	got, err := repo.Upsert(ctx, user, org, models.ViewUniboxRail, models.ViewPreferencesUpdate{Layout: layout})
	if err != nil {
		t.Fatalf("layout insert: %v", err)
	}
	if !jsonEqual(t, got.Layout, layout) {
		t.Fatalf("saved layout %s, want %s", got.Layout, layout)
	}

	// An update that does not name the layout keeps it.
	if _, err := repo.Upsert(ctx, user, org, models.ViewUniboxRail, models.ViewPreferencesUpdate{}); err != nil {
		t.Fatalf("empty update: %v", err)
	}
	read, err := repo.Get(ctx, user, org, models.ViewUniboxRail)
	if err != nil || read == nil || !jsonEqual(t, read.Layout, layout) {
		t.Fatalf("read back %v %+v", err, read)
	}

	// The column views never report a layout of their own.
	cols, err := repo.Upsert(ctx, user, org, models.ViewContacts, models.ViewPreferencesUpdate{Columns: &[]string{"company"}})
	if err != nil || cols.Layout != nil {
		t.Fatalf("contacts carries a layout: %v %s", err, cols.Layout)
	}
}

func jsonEqual(t *testing.T, a, b []byte) bool {
	t.Helper()
	var x, y any
	if err := json.Unmarshal(a, &x); err != nil {
		t.Fatalf("decode %s: %v", a, err)
	}
	if err := json.Unmarshal(b, &y); err != nil {
		t.Fatalf("decode %s: %v", b, err)
	}
	return reflect.DeepEqual(x, y)
}
