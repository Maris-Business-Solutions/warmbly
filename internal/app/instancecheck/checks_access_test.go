package instancecheck

import (
	"strings"
	"testing"

	"github.com/warmbly/warmbly/internal/config"
)

func TestRegistrationModeDescribesActivePolicy(t *testing.T) {
	for _, tc := range []struct {
		mode, title, message string
	}{
		{config.RegistrationOpen, "Public registration is open", "anyone can create an account"},
		{config.RegistrationInviteOnly, "Registration is invitation-only", "public signup is closed"},
		{config.RegistrationClosed, "Registration is closed", "workspace invitations are disabled"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			f := checkRegistrationMode(t.Context(), Deps{Policy: &config.AuthPolicy{Registration: tc.mode}}, Input{})
			if f == nil || f.Title != tc.title || f.Severity != SeverityInfo || !strings.Contains(f.Message, tc.message) {
				t.Fatalf("incorrect registration finding: %+v", f)
			}
			if !strings.Contains(f.Message, "DISABLE_REGISTRATION="+tc.mode) {
				t.Errorf("finding does not name the effective setting: %s", f.Message)
			}
		})
	}
}

func TestRegistrationModeExplainsSSOOverride(t *testing.T) {
	f := checkRegistrationMode(t.Context(), Deps{Policy: &config.AuthPolicy{
		Registration: config.RegistrationClosed, SSOAutoProvision: true,
	}}, Input{})
	if f == nil || !strings.Contains(f.Message, "SSO_AUTO_PROVISION=true") {
		t.Fatalf("SSO provisioning exception was not reported: %+v", f)
	}
}
