package handler

import (
	"testing"

	"github.com/warmbly/warmbly/internal/app/aitools"
	"github.com/warmbly/warmbly/internal/models"
)

func TestMCPNeedsAIAgent(t *testing.T) {
	cases := []struct {
		name string
		inv  aitools.Invocation
		want bool
	}{
		{"key without AI_AGENT", aitools.Invocation{IsAPIKey: true, APIPerms: models.APIPermReadContacts}, false},
		{"key with AI_AGENT", aitools.Invocation{IsAPIKey: true, APIPerms: models.APIPermAIAgent}, true},
		{"oauth member without use_ai", aitools.Invocation{IsAPIKey: true, ActsForMember: true, APIPerms: models.APIPermAIAgent, OrgPerms: models.PermViewContacts}, false},
		{"oauth member with use_ai", aitools.Invocation{IsAPIKey: true, ActsForMember: true, APIPerms: models.APIPermAIAgent, OrgPerms: models.PermUseAI}, true},
	}
	for _, tc := range cases {
		if got := mcpAllowed(tc.inv); got != tc.want {
			t.Errorf("%s: mcpAllowed = %v, want %v", tc.name, got, tc.want)
		}
	}
}
