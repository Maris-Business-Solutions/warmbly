package models

import "testing"

func TestEveryAPIPermissionNamesTheRoleItNeeds(t *testing.T) {
	var covered uint64
	for _, p := range AllAPIPermissions {
		if _, ok := apiPermissionRoleNeeds[p.Value]; !ok {
			t.Errorf("%s has no role requirement", p.Name)
		}
		covered |= p.Value
	}
	if covered != AllAPIPermissionsMask {
		t.Fatalf("AllAPIPermissions covers %b, mask is %b", covered, AllAPIPermissionsMask)
	}
}

func TestAPIPermissionsForMember(t *testing.T) {
	viewer := APIPermissionsForMember(GetRolePermissions(RoleViewer), false)
	for _, bit := range []uint64{APIPermReadCampaigns, APIPermReadContacts, APIPermReadAnalytics, APIPermReadEmails, APIPermRealtimeSubscribe} {
		if viewer&bit == 0 {
			t.Errorf("viewer should delegate %b", bit)
		}
	}
	for _, bit := range []uint64{APIPermWriteCampaigns, APIPermWriteContacts, APIPermSendCampaigns, APIPermAPIKeys, APIPermWebhooks, APIPermReadUnibox, APIPermAIResearch} {
		if viewer&bit != 0 {
			t.Errorf("viewer must not delegate %b", bit)
		}
	}

	manager := APIPermissionsForMember(GetRolePermissions(RoleManager), false)
	if manager&APIPermSendCampaigns == 0 || manager&APIPermWriteContacts == 0 {
		t.Error("manager should delegate send_campaigns and write_contacts")
	}
	if manager&(APIPermAPIKeys|APIPermWebhooks|APIPermWarmupRouting) != 0 {
		t.Error("manager must not delegate api_keys, webhooks or warmup_routing")
	}

	if got := APIPermissionsForMember(0, true); got != AllAPIPermissionsMask {
		t.Errorf("owner = %b, want every scope", got)
	}
	if got := APIPermissionsForMember(0, false); got != APIPermRealtimeSubscribe {
		t.Errorf("no permissions = %b, want only realtime_subscribe", got)
	}
	if got := APIPermissionsFor(nil); got != 0 {
		t.Errorf("nil member = %b, want 0", got)
	}
	if got := APIPermissionsFor(&OrganizationMember{Role: string(RoleOwner)}); got != AllAPIPermissionsMask {
		t.Errorf("owner member = %b, want every scope", got)
	}
	if AppGrantableScopes&APIPermAPIKeys != 0 {
		t.Error("apps must never be granted api_keys")
	}
}
