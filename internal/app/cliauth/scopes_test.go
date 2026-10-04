package cliauth

import (
	"testing"

	"github.com/warmbly/warmbly/internal/models"
)

func TestMemberScopesFollowTheRole(t *testing.T) {
	owner := &models.OrganizationMember{Role: string(models.RoleOwner)}
	if got := memberScopes(owner); got != models.AllAPIPermissionsMask {
		t.Fatalf("owner = %b, want every scope", got)
	}

	viewer := &models.OrganizationMember{Role: "viewer", Permissions: models.RolePermissions[models.RoleViewer]}
	got := memberScopes(viewer)
	if got&models.APIPermFullAccess&^models.APIPermReadOnly&^models.APIPermRealtimeSubscribe != 0 {
		t.Fatalf("viewer can grant beyond reading: %v", models.APIScopeNames(got))
	}
	if got&models.APIPermReadUnibox != 0 {
		t.Fatal("viewer can grant the unibox without access to it")
	}

	manager := &models.OrganizationMember{Role: "manager", Permissions: models.RolePermissions[models.RoleManager]}
	got = memberScopes(manager)
	for _, bit := range []uint64{models.APIPermAPIKeys, models.APIPermWebhooks, models.APIPermWarmupRouting} {
		if got&bit != 0 {
			t.Fatalf("manager can grant %v", models.APIScopeNames(bit))
		}
	}
	if got&models.APIPermSendCampaigns == 0 || got&models.APIPermIntegrations == 0 {
		t.Fatalf("manager lost scopes the role holds: %v", models.APIScopeNames(got))
	}

	if memberScopes(nil) != 0 {
		t.Fatal("a non-member can grant scopes")
	}
}
