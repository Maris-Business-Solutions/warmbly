package instancecheck

import (
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

func TestAccessFindingsAndExpiredInvitationCleanup(t *testing.T) {
	dsn := os.Getenv("INSTANCE_CHECK_TEST_DB")
	if dsn == "" {
		t.Skip("set INSTANCE_CHECK_TEST_DB to run isolated PostgreSQL regressions")
	}
	ctx := t.Context()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	schema := pgx.Identifier{"instancecheck_" + strings.ReplaceAll(uuid.NewString(), "-", "")}.Sanitize()
	if _, err := db.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := db.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `
		CREATE TABLE users (email text, admin_permissions bigint);
		CREATE TABLE organization_invitations (email text, expires_at timestamptz);
		INSERT INTO organization_invitations VALUES
			('expired@example.com', NOW() - INTERVAL '1 hour'),
			('active@example.com', NOW() + INTERVAL '1 hour');
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO users VALUES ('owner@example.com', $1), ('reader@example.com', $2)`,
		int64(models.AdminPermGrantAdminAccess|models.AdminPermViewUsers), int64(models.AdminPermViewAnalytics)); err != nil {
		t.Fatal(err)
	}
	d := Deps{DB: pool}
	if f := checkSinglePlatformAdmin(ctx, d, Input{}); f == nil || !strings.Contains(f.Message, "owner@example.com") {
		t.Fatalf("read-only admin hid the single grant-capable admin: %+v", f)
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET admin_permissions = $1 WHERE email = 'reader@example.com'`,
		int64(models.AdminPermGrantAdminAccess)); err != nil {
		t.Fatal(err)
	}
	if f := checkSinglePlatformAdmin(ctx, d, Input{}); f != nil {
		t.Fatalf("two grant-capable admins triggered a finding: %+v", f)
	}
	if f := checkExpiredInvitations(ctx, d, Input{}); f == nil || !strings.HasPrefix(f.Message, "1 expired") {
		t.Fatalf("expired invitations were not counted accurately: %+v", f)
	}
	if err := repository.NewOrganizationRepository(pool).DeleteExpiredInvitations(ctx); err != nil {
		t.Fatal(err)
	}
	if f := checkExpiredInvitations(ctx, d, Input{}); f != nil {
		t.Fatalf("finding remained after cleanup: %+v", f)
	}
	var email string
	if err := pool.QueryRow(ctx, `SELECT email FROM organization_invitations`).Scan(&email); err != nil || email != "active@example.com" {
		t.Fatalf("active invitation was changed: email=%q, err=%v", email, err)
	}
	var users int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&users); err != nil || users != 2 {
		t.Fatalf("accounts were changed: count=%d, err=%v", users, err)
	}
}
