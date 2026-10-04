package orgtransfer

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

type fakePublicURLs struct{}

func (fakePublicURLs) PublicURL(key string) string { return "https://cdn.example.com/" + key }

func rowOf(t *testing.T, v map[string]any) map[string]json.RawMessage {
	t.Helper()
	out := map[string]json.RawMessage{}
	for k, val := range v {
		enc, err := json.Marshal(val)
		if err != nil {
			t.Fatal(err)
		}
		out[k] = enc
	}
	return out
}

func TestImportedAppPassesTheAppWriteRules(t *testing.T) {
	org := uuid.New()
	id := uuid.New()
	ownLogo := "https://cdn.example.com/oauth-app-logos/" + org.String() + "/abc.png"
	env := &ruleEnv{orgID: org, logos: fakePublicURLs{}, heldApps: map[uuid.UUID]string{}}
	row := rowOf(t, map[string]any{
		"id":                      id,
		"name":                    "Visit evil.example.com now",
		"website_url":             "javascript:alert(1)",
		"logo_url":                "https://cdn.example.com/oauth-app-logos/" + uuid.NewString() + "/abc.png",
		"redirect_uris":           []string{"https://app.example.com/cb", "javascript:alert(1)", "http://evil.example.com/cb"},
		"allowed_webhook_domains": []string{"hooks.example.com"},
		"webhook_url":             "https://elsewhere.example.net/hook",
		"webhook_events":          []string{"email.sent"},
		"suspended_at":            "2026-01-01T00:00:00Z",
	})
	cleanImportedApp(env, row)

	if got := jsonString(row["name"]); got != "Imported app" {
		t.Errorf("name = %q", got)
	}
	if got := jsonString(row["website_url"]); got != "" {
		t.Errorf("website_url = %q", got)
	}
	if got := jsonString(row["logo_url"]); got != "" {
		t.Errorf("another workspace's logo kept: %q", got)
	}
	if got := jsonStrings(row["redirect_uris"]); len(got) != 1 || got[0] != "https://app.example.com/cb" {
		t.Errorf("redirect_uris = %v", got)
	}
	if got := jsonString(row["webhook_url"]); got != "" {
		t.Errorf("webhook outside its allowed domains kept: %q", got)
	}
	if env.heldApps[id] != heldAppSourceSuspended {
		t.Errorf("a suspended app was not held")
	}

	own := rowOf(t, map[string]any{"id": uuid.New(), "name": "Acme Sync", "logo_url": ownLogo})
	env.developerBlocked = true
	cleanImportedApp(env, own)
	if got := jsonString(own["logo_url"]); got != ownLogo {
		t.Errorf("own logo dropped: %q", got)
	}
	if got := jsonString(own["name"]); got != "Acme Sync" {
		t.Errorf("valid name changed: %q", got)
	}
	if len(env.heldApps) != 2 {
		t.Errorf("an app imported under a developer block was not held")
	}
}

func TestImportedFormPassesTheFormWriteRules(t *testing.T) {
	row := rowOf(t, map[string]any{
		"name":            "Newsletter",
		"redirect_url":    "javascript:alert(1)",
		"design":          map[string]any{"accent_color": "red;}body{display:none"},
		"allowed_domains": []string{"example.com", "bad host"},
		"status":          "published",
	})
	cleanImportedForm(nil, row)
	if got := jsonString(row["redirect_url"]); got != "" {
		t.Errorf("redirect_url = %q", got)
	}
	if got := string(row["design"]); got != "{}" {
		t.Errorf("design = %s", got)
	}
	if got := jsonStrings(row["allowed_domains"]); len(got) != 1 || got[0] != "example.com" {
		t.Errorf("allowed_domains = %v", got)
	}
	if got := jsonString(row["status"]); got != "draft" {
		t.Errorf("a form whose allowlist lost an entry stayed %q", got)
	}

	ok := rowOf(t, map[string]any{"redirect_url": "https://example.com/thanks", "status": "published", "allowed_domains": []string{"example.com"}})
	cleanImportedForm(nil, ok)
	if got := jsonString(ok["redirect_url"]); got != "https://example.com/thanks" {
		t.Errorf("valid redirect dropped: %q", got)
	}
	if got := jsonString(ok["status"]); got != "published" {
		t.Errorf("valid form unpublished: %q", got)
	}
}
