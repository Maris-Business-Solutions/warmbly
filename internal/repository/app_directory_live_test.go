package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/warmbly/warmbly/internal/models"
)

// The community directory shows only verified listings of active apps, opens
// published ones by link, never a rejected one, and keeps every developer write
// inside the publishing organization.
//
//	WARMBLY_TEST_DB=postgres://warmbly:warmbly@localhost:15432/<db>?sslmode=disable \
//	  go test ./internal/repository/ -run LiveAppDirectory -v

type appDirFixture struct {
	pub, viewer, other uuid.UUID
	user               uuid.UUID
	app, second        uuid.UUID
	tag                string
}

func newAppDirFixture(t *testing.T, pool *pgxpool.Pool) *appDirFixture {
	t.Helper()
	ctx := context.Background()
	f := &appDirFixture{pub: uuid.New(), viewer: uuid.New(), other: uuid.New(), user: uuid.New(), app: uuid.New(), second: uuid.New()}
	f.tag = f.pub.String()[:8]
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("fixture %q: %v", sql[:min(60, len(sql))], err)
		}
	}
	exec(`INSERT INTO users (id, first_name, last_name, email, password_hash) VALUES ($1, 'Dir', 'Live', $2, 'x')`,
		f.user, "appdir-"+f.tag+"@fixture.invalid")
	for i, org := range []uuid.UUID{f.pub, f.viewer, f.other} {
		exec(`INSERT INTO organizations (id, name, slug, owner_user_id) VALUES ($1, $2, $3, $4)`,
			org, "Org "+string(rune('A'+i)), "appdir-"+f.tag+"-"+string(rune('a'+i)), f.user)
	}
	for _, app := range []uuid.UUID{f.app, f.second} {
		exec(`INSERT INTO oauth_applications (id, organization_id, created_by, name, client_id, scopes)
		      VALUES ($1, $2, $3, $4, $5, 3)`, app, f.pub, f.user, "Acme "+app.String()[:4], "wmcid_"+app.String())
	}
	t.Cleanup(func() {
		c := context.Background()
		orgs := []uuid.UUID{f.pub, f.viewer, f.other}
		for _, sql := range []string{
			`DELETE FROM oauth_access_grants WHERE organization_id = ANY($1)`,
			`DELETE FROM oauth_applications WHERE organization_id = ANY($1)`,
			`DELETE FROM organizations WHERE id = ANY($1)`,
		} {
			if _, err := pool.Exec(c, sql, orgs); err != nil {
				t.Errorf("cleanup %q: %v", sql, err)
			}
		}
		if _, err := pool.Exec(c, `DELETE FROM users WHERE id = $1`, f.user); err != nil {
			t.Errorf("cleanup user: %v", err)
		}
	})
	return f
}

func (f *appDirFixture) grant(t *testing.T, pool *pgxpool.Pool, app, org uuid.UUID, revoked bool) {
	t.Helper()
	var revokedAt *time.Time
	if revoked {
		now := time.Now()
		revokedAt = &now
	}
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO oauth_access_grants (application_id, organization_id, user_id, scopes, access_token_hash, access_expires_at, revoked_at)
		 VALUES ($1, $2, $3, 1, $4, now() + interval '1 hour', $5)`,
		app, org, f.user, uuid.NewString(), revokedAt); err != nil {
		t.Fatalf("grant: %v", err)
	}
}

func (f *appDirFixture) listing(app uuid.UUID, slug string) *models.AppListing {
	return &models.AppListing{
		ApplicationID: app, OrganizationID: f.pub, Slug: slug, Tagline: "Sync replies",
		Category: "crm", InstallURL: "https://acme.example/install",
	}
}

func TestLiveAppDirectory(t *testing.T) {
	_, pool := liveContactDB(t)
	requireSchemaVersion(t, pool, 253)
	f := newAppDirFixture(t, pool)
	repo := NewAppDirectoryRepository(pool)
	ctx := context.Background()
	slug := "acme-" + f.tag

	verifiedSlugs := func() map[string]models.CommunityApp {
		t.Helper()
		apps, _, err := repo.ListVerified(ctx, f.viewer, 200, 0)
		if err != nil {
			t.Fatalf("ListVerified: %v", err)
		}
		out := map[string]models.CommunityApp{}
		for _, a := range apps {
			out[a.Slug] = a
		}
		return out
	}

	l := f.listing(f.app, slug)
	if err := repo.SaveListing(ctx, l); err != nil {
		t.Fatalf("SaveListing: %v", err)
	}
	if l.Verification != models.AppListingUnverified {
		t.Fatalf("new listing verification = %q, want unverified", l.Verification)
	}

	t.Run("an unverified listing opens by link and stays out of discovery", func(t *testing.T) {
		if _, ok := verifiedSlugs()[slug]; ok {
			t.Fatal("unverified listing was listed")
		}
		app, err := repo.GetPublished(ctx, f.viewer, slug)
		if err != nil || app == nil {
			t.Fatalf("GetPublished = %v, %v", app, err)
		}
		if app.Developer != "Org A" || app.Verification != models.AppListingUnverified || len(app.Permissions) != 2 {
			t.Fatalf("unexpected listing: %+v", app)
		}
	})

	t.Run("a slug is unique across apps", func(t *testing.T) {
		if err := repo.SaveListing(ctx, f.listing(f.second, slug)); !errors.Is(err, ErrAppListingSlugTaken) {
			t.Fatalf("duplicate slug err = %v, want ErrAppListingSlugTaken", err)
		}
	})

	t.Run("another organization cannot list or remove the app", func(t *testing.T) {
		foreign := f.listing(f.second, "foreign-"+f.tag)
		foreign.OrganizationID = f.other
		if err := repo.SaveListing(ctx, foreign); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("foreign save err = %v, want no rows", err)
		}
		if err := repo.DeleteListing(ctx, f.other, f.app); err != nil {
			t.Fatalf("foreign delete: %v", err)
		}
		if got, _ := repo.GetListing(ctx, f.pub, f.app); got == nil {
			t.Fatal("a foreign delete removed the listing")
		}
		if got, _ := repo.GetListing(ctx, f.other, f.app); got != nil {
			t.Fatal("a foreign organization read the listing")
		}
	})

	t.Run("a verified listing is listed with live installs", func(t *testing.T) {
		admin := f.user
		if err := repo.SetVerification(ctx, f.app, models.AppListingVerified, admin, ""); err != nil {
			t.Fatalf("verify: %v", err)
		}
		f.grant(t, pool, f.app, f.viewer, false)
		f.grant(t, pool, f.app, f.viewer, false)
		f.grant(t, pool, f.app, f.other, false)
		f.grant(t, pool, f.app, f.pub, true)
		got, ok := verifiedSlugs()[slug]
		if !ok {
			t.Fatal("verified listing missing from discovery")
		}
		if got.Installs != 2 || !got.Installed {
			t.Fatalf("installs = %d installed = %v, want 2 and true", got.Installs, got.Installed)
		}
	})

	t.Run("a disabled app leaves discovery and its link", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `UPDATE oauth_applications SET status = 'disabled' WHERE id = $1`, f.app); err != nil {
			t.Fatal(err)
		}
		defer func() { _, _ = pool.Exec(ctx, `UPDATE oauth_applications SET status = 'active' WHERE id = $1`, f.app) }()
		if _, ok := verifiedSlugs()[slug]; ok {
			t.Fatal("disabled app listed")
		}
		if app, _ := repo.GetPublished(ctx, f.viewer, slug); app != nil {
			t.Fatal("disabled app opened by link")
		}
	})

	t.Run("a change to the app returns it to review", func(t *testing.T) {
		if err := repo.Unverify(ctx, f.pub, f.app); err != nil {
			t.Fatalf("Unverify: %v", err)
		}
		got, _ := repo.GetListing(ctx, f.pub, f.app)
		if got == nil || got.Verification != models.AppListingUnverified {
			t.Fatalf("after Unverify: %+v", got)
		}
	})

	t.Run("a rejected listing is unreachable and an edit resubmits it", func(t *testing.T) {
		if err := repo.SetVerification(ctx, f.app, models.AppListingRejected, f.user, "wrong logo"); err != nil {
			t.Fatalf("reject: %v", err)
		}
		if app, _ := repo.GetPublished(ctx, f.viewer, slug); app != nil {
			t.Fatal("rejected listing opened by link")
		}
		edit := f.listing(f.app, slug)
		edit.Tagline = "Sync replies, fixed"
		if err := repo.SaveListing(ctx, edit); err != nil {
			t.Fatalf("resubmit: %v", err)
		}
		if edit.Verification != models.AppListingUnverified {
			t.Fatalf("resubmitted verification = %q", edit.Verification)
		}
	})

	t.Run("the admin queue filters and pages", func(t *testing.T) {
		second := f.listing(f.second, "second-"+f.tag)
		if err := repo.SaveListing(ctx, second); err != nil {
			t.Fatalf("second listing: %v", err)
		}
		rows, total, err := repo.AdminList(ctx, &models.AdminAppListingSearch{Q: f.tag, Verification: "unverified", Limit: 1})
		if err != nil {
			t.Fatalf("AdminList: %v", err)
		}
		if total != 2 || len(rows) != 1 {
			t.Fatalf("total = %d rows = %d, want 2 and 1", total, len(rows))
		}
		rest, _, err := repo.AdminList(ctx, &models.AdminAppListingSearch{Q: f.tag, Verification: "unverified", Limit: 1, Offset: 1})
		if err != nil || len(rest) != 1 || rest[0].ApplicationID == rows[0].ApplicationID {
			t.Fatalf("second page = %+v, %v", rest, err)
		}
		if rows[0].OrganizationName != "Org A" || len(rows[0].Permissions) != 2 {
			t.Fatalf("unexpected admin row: %+v", rows[0])
		}
		got, err := repo.AdminGet(ctx, f.app)
		if err != nil || got == nil || got.ReviewedByEmail == "" {
			t.Fatalf("AdminGet = %+v, %v", got, err)
		}
	})

	t.Run("unpublishing removes the listing", func(t *testing.T) {
		if err := repo.DeleteListing(ctx, f.pub, f.app); err != nil {
			t.Fatalf("DeleteListing: %v", err)
		}
		if got, _ := repo.GetListing(ctx, f.pub, f.app); got != nil {
			t.Fatal("listing survived DeleteListing")
		}
		if err := repo.SetVerification(ctx, f.app, models.AppListingVerified, f.user, ""); !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("verify missing listing err = %v, want no rows", err)
		}
	})
}
