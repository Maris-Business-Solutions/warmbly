package cliauth

import "github.com/warmbly/warmbly/internal/models"

// scopeGates mirrors the routes' RequireAccess pairs: an API scope is grantable
// by a member holding any one of the organization permissions listed for it.
// A scope with no entry needs membership only.
var scopeGates = map[uint64][]models.OrganizationPermission{
	models.APIPermReadEmails:     {models.PermViewCampaigns, models.PermManageEmails},
	models.APIPermReadCampaigns:  {models.PermViewCampaigns},
	models.APIPermReadContacts:   {models.PermViewContacts},
	models.APIPermReadUnibox:     {models.PermAccessUnibox},
	models.APIPermReadAnalytics:  {models.PermViewAnalytics},
	models.APIPermReadTemplates:  {models.PermViewCampaigns},
	models.APIPermReadCRM:        {models.PermViewContacts},
	models.APIPermReadAuditLogs:  {models.PermViewAnalytics},
	models.APIPermWriteEmails:    {models.PermManageEmails},
	models.APIPermWriteCampaigns: {models.PermManageCampaigns},
	models.APIPermWriteContacts:  {models.PermManageContacts},
	models.APIPermWriteUnibox:    {models.PermAccessUnibox},
	models.APIPermWriteTemplates: {models.PermManageCampaigns},
	models.APIPermWriteCRM:       {models.PermManageContacts},
	models.APIPermSendCampaigns:  {models.PermSendCampaigns},
	models.APIPermBulkContacts:   {models.PermManageContacts},
	models.APIPermBulkCampaigns:  {models.PermManageCampaigns},
	models.APIPermWebhooks:       {models.PermManageSettings},
	models.APIPermAPIKeys:        {models.PermManageAPIKeys},
	models.APIPermIntegrations:   {models.PermManageSettings, models.PermUseIntegrations},
	models.APIPermWarmupRouting:  {models.PermManageSettings},
	models.APIPermAIAgent:        {models.PermUseAI},
	models.APIPermAIResearch:     {models.PermManageContacts},
}

// memberScopes is every API scope a member's role lets them hand to a key.
func memberScopes(m *models.OrganizationMember) uint64 {
	if m == nil {
		return 0
	}
	var out uint64
	for bit := uint64(1); bit != 0 && bit <= models.AllAPIPermissionsMask; bit <<= 1 {
		if models.AllAPIPermissionsMask&bit == 0 {
			continue
		}
		gates, gated := scopeGates[bit]
		if !gated {
			out |= bit
			continue
		}
		for _, p := range gates {
			if m.HasPermission(p) {
				out |= bit
				break
			}
		}
	}
	return out
}
