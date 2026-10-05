package instancecheck

import (
	"testing"

	"github.com/warmbly/warmbly/internal/app/instanceconfig"
)

func TestDashboardURLDoesNotNeedToMatchAPIOrAdminHost(t *testing.T) {
	t.Setenv("APP_URL", "https://app.example.com")
	t.Setenv("API_PUBLIC_URL", "https://api.example.com")
	d := Deps{Runtime: &instanceconfig.Runtime{
		CORSOrigins: []string{"https://app.example.com", "https://admin.example.com"},
	}}
	for _, origin := range []string{"https://app.example.com", "https://admin.example.com", ""} {
		in := Input{Host: "api.example.com", Origin: origin}
		for _, c := range urlChecks() {
			switch c.id {
			case "app_url_unset", "app_url_insecure", "app_url_host_mismatch", "cors_missing_origin":
				if f := c.run(t.Context(), d, in); f != nil {
					t.Errorf("origin %q triggered %s on valid separate hosts: %s", origin, c.id, f.Message)
				}
			}
		}
	}
}

func TestDashboardURLStillChecksHTTPSAndCORS(t *testing.T) {
	t.Setenv("APP_URL", "http://app.example.com")
	if f := checkAppURLInsecure(t.Context(), Deps{}, Input{}); f == nil || f.Severity != SeverityWarning {
		t.Fatalf("insecure dashboard was not reported: %+v", f)
	}
	t.Setenv("APP_URL", "https://app.example.com")
	d := Deps{Runtime: &instanceconfig.Runtime{CORSOrigins: []string{"https://admin.example.com"}}}
	if f := checkCORSMissingOrigin(t.Context(), d, Input{Host: "api.example.com", Origin: "https://admin.example.com"}); f == nil {
		t.Fatal("dashboard missing from CORS was not reported")
	}
}
