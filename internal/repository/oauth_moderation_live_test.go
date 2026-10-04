package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/models"
)

// A suspended or disabled app's tokens stop working, blocks are found by
// workspace or by person, and the operator's actions touch only their target.
//
//	WARMBLY_TEST_DB=postgres://warmbly:warmbly@localhost:15432/<db>?sslmode=disable \
//	  go test ./internal/repository/ -run LiveOAuthModeration -v
func TestLiveOAuthModeration(t *testing.T) {
	_, pool := liveContactDB(t)
	requireSchemaVersion(t, pool, 253)
	ctx := context.Background()
	org, other, user, admin := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	app, app2 := uuid.New(), uuid.New()
	tag := org.String()[:8]
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("fixture %q: %v", sql[:min(60, len(sql))], err)
		}
	}
	for i, u := range []uuid.UUID{user, admin} {
		exec(`INSERT INTO users (id, first_name, last_name, email, password_hash) VALUES ($1, 'Mod', 'Live', $2, 'x')`,
			u, "mod-"+tag+"-"+string(rune('a'+i))+"@fixture.invalid")
	}
	for i, o := range []uuid.UUID{org, other} {
		exec(`INSERT INTO organizations (id, name, slug, owner_user_id) VALUES ($1, 'Mod Org', $2, $3)`, o, "mod-"+tag+"-"+string(rune('a'+i)), user)
		exec(`INSERT INTO organization_members (organization_id, user_id, role, accepted_at) VALUES ($1, $2, 'owner', now())`, o, user)
	}
	exec(`INSERT INTO oauth_applications (id, organization_id, created_by, name, client_id, scopes, logo_url) VALUES ($1, $2, $3, 'Mod App', $4, 1, 'https://x.example/l.png')`,
		app, org, user, "wmcid_"+app.String())
	exec(`INSERT INTO oauth_applications (id, organization_id, created_by, name, client_id, scopes) VALUES ($1, $2, $3, 'Mod App 2', $4, 1)`,
		app2, org, user, "wmcid_"+app2.String())
	exec(`INSERT INTO oauth_access_grants (application_id, organization_id, user_id, scopes, access_token_hash, refresh_token_hash, access_expires_at)
	      VALUES ($1, $2, $3, 1, $4, $5, now() + interval '1 hour')`, app, other, user, "acc-"+tag, "ref-"+tag)
	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM oauth_developer_blocks WHERE organization_id = ANY($1) OR user_id = ANY($2)`, []uuid.UUID{org, other}, []uuid.UUID{user, admin})
		_, _ = pool.Exec(c, `DELETE FROM oauth_access_grants WHERE application_id = ANY($1)`, []uuid.UUID{app, app2})
		_, _ = pool.Exec(c, `DELETE FROM oauth_applications WHERE id = ANY($1)`, []uuid.UUID{app, app2})
		_, _ = pool.Exec(c, `DELETE FROM organization_members WHERE organization_id = ANY($1)`, []uuid.UUID{org, other})
		_, _ = pool.Exec(c, `DELETE FROM organizations WHERE id = ANY($1)`, []uuid.UUID{org, other})
		_, _ = pool.Exec(c, `DELETE FROM users WHERE id = ANY($1)`, []uuid.UUID{user, admin})
	})

	repo := NewOAuthRepository(pool)
	mod := NewOAuthAdminRepository(pool)
	tokenWorks := func() bool {
		t.Helper()
		g, err := repo.GetGrantByAccessTokenHash(ctx, "acc-"+tag)
		if err != nil {
			t.Fatalf("access lookup: %v", err)
		}
		r, err := repo.GetGrantByRefreshTokenHash(ctx, "ref-"+tag)
		if err != nil {
			t.Fatalf("refresh lookup: %v", err)
		}
		if (g == nil) != (r == nil) {
			t.Fatalf("access and refresh disagree: %v %v", g, r)
		}
		return g != nil
	}

	t.Run("a suspended app's tokens stop working until unsuspended", func(t *testing.T) {
		if !tokenWorks() {
			t.Fatal("token of a usable app did not work")
		}
		if changed, err := mod.Suspend(ctx, app, admin, "phishing"); err != nil || !changed {
			t.Fatalf("Suspend = %v, %v", changed, err)
		}
		if changed, _ := mod.Suspend(ctx, app, admin, "again"); changed {
			t.Fatal("a second suspend reported a change")
		}
		if tokenWorks() {
			t.Fatal("token of a suspended app still works")
		}
		a, _ := repo.GetApplication(ctx, org, app)
		if a.SuspendedAt == nil || a.SuspendedReason != "phishing" || a.Usable() {
			t.Fatalf("suspended app = %+v", a)
		}
		if err := mod.Unsuspend(ctx, app); err != nil {
			t.Fatal(err)
		}
		if !tokenWorks() {
			t.Fatal("token did not come back after unsuspend")
		}
	})

	t.Run("an app its owner disabled accepts no token", func(t *testing.T) {
		exec(`UPDATE oauth_applications SET status = 'disabled' WHERE id = $1`, app)
		defer exec(`UPDATE oauth_applications SET status = 'active' WHERE id = $1`, app)
		if tokenWorks() {
			t.Fatal("token of a disabled app still works")
		}
	})

	t.Run("blocks are found by workspace or by person", func(t *testing.T) {
		if b, err := repo.DeveloperBlock(ctx, org, user); err != nil || b != nil {
			t.Fatalf("unblocked lookup = %v, %v", b, err)
		}
		b, err := mod.CreateBlock(ctx, &org, nil, admin, "spam apps")
		if err != nil || b.OrganizationName != "Mod Org" {
			t.Fatalf("CreateBlock = %+v, %v", b, err)
		}
		if _, err := mod.CreateBlock(ctx, &org, nil, admin, "twice"); !errors.Is(err, ErrDeveloperBlockExists) {
			t.Fatalf("duplicate block err = %v", err)
		}
		if got, _ := repo.DeveloperBlock(ctx, org, uuid.New()); got == nil || got.Reason != "spam apps" {
			t.Fatalf("workspace block not found: %+v", got)
		}
		if got, _ := repo.DeveloperBlock(ctx, other, uuid.New()); got != nil {
			t.Fatal("a workspace block leaked to another workspace")
		}
		pb, err := mod.CreateBlock(ctx, nil, &user, admin, "person")
		if err != nil {
			t.Fatal(err)
		}
		if got, _ := repo.DeveloperBlock(ctx, other, user); got == nil {
			t.Fatal("a person block was not found in another workspace")
		}
		if got, _ := repo.DeveloperBlock(ctx, other, uuid.Nil); got != nil {
			t.Fatal("an unknown person matched a person block")
		}
		list, _ := mod.ListBlocks(ctx)
		found := 0
		for _, x := range list {
			if x.ID == b.ID || x.ID == pb.ID {
				found++
			}
		}
		if found != 2 {
			t.Fatalf("ListBlocks found %d of 2", found)
		}
		for _, id := range []uuid.UUID{b.ID, pb.ID} {
			if ok, err := mod.DeleteBlock(ctx, id); err != nil || !ok {
				t.Fatalf("DeleteBlock = %v, %v", ok, err)
			}
		}
	})

	t.Run("suspending a workspace's apps, revoking and clearing logos", func(t *testing.T) {
		ids, err := mod.SuspendOrgApps(ctx, org, admin, "bulk")
		if err != nil || len(ids) != 2 {
			t.Fatalf("SuspendOrgApps = %v, %v", ids, err)
		}
		rows, total, err := mod.ListApps(ctx, &models.AdminOAuthAppSearch{Q: tag, Status: "suspended", Limit: 10})
		if err != nil {
			t.Fatalf("ListApps: %v", err)
		}
		if total != 2 || len(rows) != 2 {
			t.Fatalf("suspended fixture apps listed = %d (total %d), want 2", len(rows), total)
		}
		if active, _, _ := mod.ListApps(ctx, &models.AdminOAuthAppSearch{Q: tag, Status: "active", Limit: 10}); len(active) != 0 {
			t.Fatalf("suspended apps listed as active: %d", len(active))
		}
		got, err := mod.GetApp(ctx, app)
		if err != nil || got == nil || got.SuspendedAt == nil || got.Installs != 1 || got.CreatedByEmail == "" {
			t.Fatalf("GetApp = %+v, %v", got, err)
		}
		n, err := mod.RevokeAllGrants(ctx, app)
		if err != nil || n != 1 {
			t.Fatalf("RevokeAllGrants = %d, %v", n, err)
		}
		if err := mod.ClearLogo(ctx, app); err != nil {
			t.Fatal(err)
		}
		if got, _ := mod.GetApp(ctx, app); got.LogoURL != "" || got.Installs != 0 {
			t.Fatalf("after revoke and clear: %+v", got)
		}
	})

	t.Run("the owner can set and clear the logo of its own app only", func(t *testing.T) {
		if err := repo.UpdateApplicationLogo(ctx, other, app2, "https://x.example/new.png"); err == nil {
			t.Fatal("another workspace set the logo")
		}
		if err := repo.UpdateApplicationLogo(ctx, org, app2, "https://x.example/new.png"); err != nil {
			t.Fatal(err)
		}
		if a, _ := repo.GetApplication(ctx, org, app2); a.LogoURL != "https://x.example/new.png" {
			t.Fatalf("logo = %q", a.LogoURL)
		}
	})
}
