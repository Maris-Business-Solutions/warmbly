package oauth

import "testing"

func TestAppNameFollowsTheNamingRules(t *testing.T) {
	if got, err := appName("  Acme   Sync "); err != nil || got != "Acme Sync" {
		t.Fatalf("appName = %q, %v", got, err)
	}
	for _, bad := range []string{"", "Verify at evil.example", "https://acme.com", "Acme ‮ppa", "<b>Acme</b>"} {
		if _, err := appName(bad); err == nil {
			t.Errorf("appName(%q) accepted", bad)
		}
	}
}

func TestAppWebsite(t *testing.T) {
	for _, ok := range []string{"", "https://acme.com", "http://acme.com/path"} {
		if _, err := appWebsite(ok); err != nil {
			t.Errorf("appWebsite(%q) refused: %v", ok, err)
		}
	}
	for _, bad := range []string{"javascript:alert(1)", "data:text/html,x", "acme.com", "https://user:pw@acme.com", "ftp://acme.com"} {
		if _, err := appWebsite(bad); err == nil {
			t.Errorf("appWebsite(%q) accepted", bad)
		}
	}
	if dcrWebsite("javascript:alert(1)") != "" {
		t.Fatal("a self-registered client kept an unsafe website")
	}
}
