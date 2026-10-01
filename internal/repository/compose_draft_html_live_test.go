package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// A compose draft keeps the HTML body a template gave it, and a later save
// that drops back to plain text clears it.
//
//	WARMBLY_TEST_DB=postgres://warmbly:warmbly@localhost:15432/<db>?sslmode=disable \
//	  go test ./internal/repository/ -run LiveComposeDraftHTML -v
func TestLiveComposeDraftHTML(t *testing.T) {
	handle, pool := liveContactDB(t)
	ctx := context.Background()
	user, org := uuid.New(), uuid.New()
	tag := org.String()[:8]
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO users (id, first_name, last_name, email, password_hash) VALUES ($1, 'Draft', 'Live', $2, 'x')`, []any{user, "drafts-" + tag + "@fixture.invalid"}},
		{`INSERT INTO organizations (id, name, slug, owner_user_id) VALUES ($1, 'Drafts', $2, $3)`, []any{org, "drafts-" + tag, user}},
	} {
		if _, err := pool.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatalf("fixture: %v", err)
		}
	}
	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM compose_drafts WHERE organization_id = $1`, org)
		_, _ = pool.Exec(c, `DELETE FROM organizations WHERE id = $1`, org)
		_, _ = pool.Exec(c, `DELETE FROM users WHERE id = $1`, user)
	})

	repo := NewComposeRepository(handle)
	html := `<p>Here is our <a href="https://example.com/brochure.pdf">brochure</a>.</p>`
	d := &ComposeDraft{ID: uuid.New(), To: []string{"a@example.com"}, CC: []string{}, BCC: []string{}, Subject: "Brochure", Body: "Here is our brochure", BodyHTML: html}
	if err := repo.UpsertDraft(ctx, user, org, d); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := repo.ListDrafts(ctx, user, org)
	if err != nil || len(got) != 1 {
		t.Fatalf("list: %v %d", err, len(got))
	}
	if got[0].BodyHTML != html || got[0].Body != d.Body {
		t.Fatalf("draft: body %q html %q", got[0].Body, got[0].BodyHTML)
	}

	d.BodyHTML = ""
	if err := repo.UpsertDraft(ctx, user, org, d); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	got, err = repo.ListDrafts(ctx, user, org)
	if err != nil || len(got) != 1 || got[0].BodyHTML != "" {
		t.Fatalf("plain re-save kept html: %v %+v", err, got)
	}
}
