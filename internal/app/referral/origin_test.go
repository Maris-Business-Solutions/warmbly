package referral

import (
	"context"
	"testing"

	"github.com/warmbly/warmbly/internal/config"
)

func TestReferralShareLinkUsesTrustedDashboard(t *testing.T) {
	t.Setenv("APP_URL", "https://app.warmbly.com")
	t.Setenv("APP_ORIGIN", "")
	t.Setenv("CORS_ALLOW_ORIGINS", "https://tac-security-assessment.warmbly.com")
	s := &service{shareBase: "https://app.warmbly.com"}
	for _, tt := range []struct{ origin, want string }{
		{"https://tac-security-assessment.warmbly.com", "https://tac-security-assessment.warmbly.com"},
		{"", "https://app.warmbly.com"},
		{"https://evil.example.com", "https://app.warmbly.com"},
	} {
		ctx := config.WithDashboardOrigin(context.Background(), tt.origin)
		if got := s.shareURL(ctx, "CODE1234"); got != tt.want+"/auth/register?ref=CODE1234" {
			t.Fatalf("share link = %q", got)
		}
	}
}
