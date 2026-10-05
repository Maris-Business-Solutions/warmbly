package repository

import (
	"context"
	"net/mail"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/models"
)

func TestLiveTesterWorkspaceProvisioning(t *testing.T) {
	db, pool := liveContactDB(t)
	requireSchemaVersion(t, pool, 264)
	ctx := context.Background()
	users := NewUserRepostory(db, nil)
	orgs := NewOrganizationRepository(pool)
	until := time.Now().Add(24 * time.Hour).Truncate(time.Microsecond)
	u, err := users.CreateExemptUser(ctx, &mail.Address{Address: uuid.NewString() + "@example.test"}, "hash", "review", nil, until)
	if err != nil {
		t.Fatal(err)
	}
	slug := uuid.NewString()
	org := &models.Organization{ID: uuid.New(), OwnerUserID: u.ID, Name: "Test workspace", Slug: &slug}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM organizations WHERE id = $1`, org.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, u.ID)
	})
	if u.OnboardingCompletedAt == nil {
		t.Fatal("created reviewer must be onboarded")
	}
	stored, err := users.GetUser(ctx, u.ID)
	if err != nil || stored.OnboardingCompletedAt == nil {
		t.Fatalf("stored reviewer must be onboarded: %v", err)
	}
	if err := orgs.Create(ctx, org); err != nil {
		t.Fatal(err)
	}
	// An invalid grant actor fails after categorization; the whole grant must roll back.
	if err := orgs.ProvisionTesterWorkspace(ctx, org.ID, u.ID, uuid.New(), "review", until); err == nil {
		t.Fatal("expected a foreign-key failure")
	}
	rolledBack, err := orgs.GetByID(ctx, org.ID)
	if err != nil || rolledBack.Category != models.OrganizationCategoryStandard {
		t.Fatalf("failed provisioning changed category: %v, %v", rolledBack, err)
	}
	var credits int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM credit_ledger WHERE org_id = $1`, org.ID).Scan(&credits); err != nil || credits != 0 {
		t.Fatalf("failed provisioning minted credits: %d, %v", credits, err)
	}
	if err := orgs.ProvisionTesterWorkspace(ctx, org.ID, u.ID, u.ID, "review", until); err != nil {
		t.Fatal(err)
	}
	provisioned, err := orgs.GetByID(ctx, org.ID)
	if err != nil || provisioned.Category != models.OrganizationCategoryTest {
		t.Fatalf("test category missing: %v, %v", provisioned, err)
	}
	adminDetail, err := orgs.GetOrganizationAdminDetail(ctx, org.ID)
	if err != nil || adminDetail.Category != models.OrganizationCategoryTest {
		t.Fatalf("admin category missing: %v, %v", adminDetail, err)
	}
	list, err := orgs.SearchOrganizationsForAdmin(ctx, &models.AdminOrgSearch{Query: slug, Limit: 10})
	if err != nil || len(list.Data) != 1 || list.Data[0].Category != models.OrganizationCategoryTest {
		t.Fatalf("admin list category missing: %v, %v", list, err)
	}
	subs := NewSubscriptionRepository(pool)
	sub, err := subs.GetByOrganizationID(ctx, org.ID)
	if err != nil || !sub.HasProductPlan() || sub.EffectivePlanID() != models.TestPlanID || !sub.ManagedUntil.Equal(until) {
		t.Fatalf("test plan or expiry missing: %v, %v", sub, err)
	}
	if err := orgs.ProvisionTesterWorkspace(ctx, org.ID, u.ID, u.ID, "retry", until); err == nil {
		t.Fatal("reprovisioning an existing workspace must fail")
	}
	var txns int
	if err := pool.QueryRow(ctx, `SELECT balance FROM credit_ledger WHERE org_id = $1`, org.ID).Scan(&credits); err != nil || credits != 100 {
		t.Fatalf("test credits = %d, %v", credits, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM credit_ledger_transactions WHERE org_id = $1 AND reason = 'tester_grant'`, org.ID).Scan(&txns); err != nil || txns != 1 {
		t.Fatalf("test grant audit = %d, %v", txns, err)
	}
	if cleared, err := users.RevokeTester(ctx, u.ID); err != nil || !cleared {
		t.Fatalf("revoke = %v, %v", cleared, err)
	}
	sub, err = subs.GetByOrganizationID(ctx, org.ID)
	if err != nil || sub.HasProductPlan() {
		t.Fatalf("revocation left paid access: %v, %v", sub, err)
	}
}

func TestLiveTesterWorkspaceBackfill(t *testing.T) {
	_, pool := liveContactDB(t)
	requireSchemaVersion(t, pool, 264)
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	readMigration := func(direction string) string {
		t.Helper()
		data, err := os.ReadFile("../infrastructure/db/migrations/000264_tester_workspaces." + direction + ".sql")
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	exec(readMigration("down"))

	for _, tc := range []struct {
		name                                                                      string
		joined, expired, revoked, cli, legacy, paid, managed, missingSubscription bool
		category                                                                  string
		grant, onboarded                                                          bool
	}{
		{name: "dedicated", category: "test", grant: true, onboarded: true},
		{name: "dedicated without subscription", missingSubscription: true, category: "test", grant: true, onboarded: true},
		{name: "existing", joined: true, category: "standard", onboarded: true},
		{name: "expired", expired: true, category: "test", onboarded: true},
		{name: "revoked", revoked: true, category: "test"},
		{name: "CLI exemption", cli: true, legacy: true, category: "standard"},
		{name: "legacy dedicated", legacy: true, category: "test", grant: true, onboarded: true},
		{name: "legacy expired", legacy: true, expired: true, category: "test", onboarded: true},
		{name: "Stripe workspace", paid: true, category: "test", onboarded: true},
		{name: "managed workspace", managed: true, category: "test", onboarded: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exec := func(sql string, args ...any) {
				t.Helper()
				if _, err := tx.Exec(ctx, sql, args...); err != nil {
					t.Fatal(err)
				}
			}
			userID, orgID := uuid.New(), uuid.New()
			until := time.Now().Add(time.Hour)
			if tc.expired {
				until = time.Now().Add(-time.Hour)
			}
			var expiry *time.Time
			if !tc.legacy && !tc.revoked {
				expiry = &until
			}
			exec(`INSERT INTO users (id, first_name, last_name, email, password_hash, login_code_exempt, login_code_exempt_reason, login_code_exempt_at, password_expires_at)
				VALUES ($1, 'Reviewer', '', $2, 'hash', $3, 'review', now(), $4)`, userID, userID.String()+"@example.test", !tc.revoked, expiry)
			exec(`INSERT INTO organizations (id, name, owner_user_id) VALUES ($1, 'Reviewer workspace', $2)`, orgID, userID)
			if !tc.cli {
				exec(`INSERT INTO admin_audit_logs (admin_user_id, action, target_type, target_id, details)
					VALUES ($1, 'create_tester', 'user', $1, jsonb_build_object('organization_id', $2::text, 'joined_existing', $3::boolean))`, userID, orgID.String(), tc.joined)
			}
			if tc.legacy {
				exec(`UPDATE admin_audit_logs SET details = details - 'joined_existing' WHERE target_id = $1`, userID)
				if tc.expired {
					exec(`UPDATE admin_audit_logs SET created_at = now() - interval '40 days' WHERE target_id = $1`, userID)
				}
			}
			var stripeID *string
			if tc.paid {
				id := "sub_" + orgID.String()
				stripeID = &id
			}
			if !tc.missingSubscription {
				exec(`INSERT INTO subscriptions (user_id, organization_id, plan_id, stripe_customer_id, stripe_subscription_id)
					VALUES ($1, $2, '00000000-0000-0000-0000-000000000001', '', $3)`, userID, orgID, stripeID)
			}
			if tc.managed {
				exec(`UPDATE subscriptions SET managed_at = now(), managed_plan_id = plan_id, managed_reason = 'operator grant' WHERE organization_id = $1`, orgID)
			}
			exec(`INSERT INTO credit_ledger (org_id, balance, purchased_balance) VALUES ($1, 25, 75)`, orgID)
			exec(readMigration("up"))
			var category string
			var managedID *uuid.UUID
			var completed, storedExpiry *time.Time
			var balance, purchased int
			if err := tx.QueryRow(ctx, `SELECT o.category, s.managed_plan_id, u.onboarding_completed_at, u.password_expires_at, c.balance, c.purchased_balance
				FROM organizations o JOIN users u ON u.id = o.owner_user_id
				JOIN subscriptions s ON s.organization_id = o.id JOIN credit_ledger c ON c.org_id = o.id WHERE o.id = $1`, orgID).
				Scan(&category, &managedID, &completed, &storedExpiry, &balance, &purchased); err != nil {
				t.Fatal(err)
			}
			granted := managedID != nil && *managedID == models.TestPlanID
			wantBalance := 25
			if tc.grant {
				wantBalance += 100
			}
			if category != tc.category || granted != tc.grant || (completed != nil) != tc.onboarded || balance != wantBalance || purchased != 75 {
				t.Fatalf("category=%s granted=%v onboarded=%v balance=%d purchased=%d", category, granted, completed != nil, balance, purchased)
			}
			if tc.legacy && !tc.cli && storedExpiry == nil {
				t.Fatal("legacy reviewer did not receive a bounded password lifetime")
			}
			if tc.legacy && tc.expired && storedExpiry.After(time.Now()) {
				t.Fatal("upgrade renewed an expired legacy reviewer")
			}
			if tc.managed && (managedID == nil || *managedID == models.TestPlanID) {
				t.Fatal("migration overwrote an operator grant")
			}
			exec(readMigration("down"))
		})
	}
}
