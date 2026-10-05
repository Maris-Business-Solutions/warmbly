package config

import "testing"

func TestDashboardOrigin(t *testing.T) {
	t.Setenv("APP_URL", "https://app.example.com")
	t.Setenv("APP_ORIGIN", "https://primary.example.com")
	t.Setenv("CORS_ALLOW_ORIGINS", "https://app.example.com, https://assessment.example.com,http://localhost:5173")
	for _, value := range []string{"https://app.example.com", "https://primary.example.com", "https://assessment.example.com", "http://localhost:5173"} {
		if got := DashboardOrigin(value); got != value {
			t.Errorf("DashboardOrigin(%q) = %q", value, got)
		}
	}
	for _, value := range []string{"", "null", "*", "https://evil.example.com", "https://assessment.example.com.evil.test", "http://assessment.example.com", "https://assessment.example.com:444", "https://assessment.example.com/", "https://assessment.example.com/redirect", "https://assessment.example.com?x=1", "https://assessment.example.com?", "https://assessment.example.com#fragment", "https://user@assessment.example.com", "javascript:alert(1)"} {
		if got := DashboardOrigin(value); got != "" {
			t.Errorf("DashboardOrigin(%q) = %q, want empty", value, got)
		}
	}
	t.Setenv("CORS_ALLOW_ORIGINS", "*")
	if got := DashboardOrigin("https://evil.example.com"); got != "" {
		t.Fatal("wildcard must not allow an OAuth return origin")
	}
}
