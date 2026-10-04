package email

import (
	"net/url"
	"strings"
	"testing"

	"github.com/warmbly/warmbly/internal/config"
	"github.com/warmbly/warmbly/internal/models"
	"golang.org/x/oauth2"
)

func authQuery(t *testing.T, provider models.InboxProvider, loginHint string) url.Values {
	t.Helper()
	cfg := &oauth2.Config{
		ClientID:    "client",
		RedirectURL: "https://api.example.com/addresses/callback",
		Endpoint:    oauth2.Endpoint{AuthURL: "https://provider.example.com/authorize"},
	}
	raw := cfg.AuthCodeURL("state-value", authCodeOptions(provider, loginHint)...)
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse authorize url: %v", err)
	}
	return u.Query()
}

// prompt=consent makes Entra ID re-run the consent eligibility check for the
// signing user instead of honouring the tenant's existing admin grant, so every
// non-admin is refused with AADSTS90095 no matter how the grant is built
// (issue #409). Microsoft issues the refresh token off the offline_access scope,
// so nothing is lost by dropping it.
func TestAuthCodeOptions_OutlookNeverForcesConsent(t *testing.T) {
	q := authQuery(t, models.InboxProviderOutlook, "")

	if got := q.Get("prompt"); got != "select_account" {
		t.Errorf("prompt = %q, want select_account so a tenant-wide admin grant is honoured", got)
	}
	if q.Has("access_type") {
		t.Errorf("access_type = %q, a Google-only parameter Microsoft has no use for", q.Get("access_type"))
	}
}

// Google re-issues a refresh token only when the consent screen is forced, and
// issues one at all only under access_type=offline. Losing either turns a
// reconnect into a mailbox that stops sending an hour later.
func TestAuthCodeOptions_GoogleKeepsOfflineConsent(t *testing.T) {
	q := authQuery(t, models.InboxProviderGoogle, "")

	if got := q.Get("prompt"); got != "consent" {
		t.Errorf("prompt = %q, want consent so a repeat authorization still returns a refresh token", got)
	}
	if got := q.Get("access_type"); got != "offline" {
		t.Errorf("access_type = %q, want offline", got)
	}
}

// A reconnect names the mailbox it is renewing so the picker offers that
// account first; the finish leg still refuses tokens for any other address.
func TestAuthCodeOptions_LoginHintOnlyOnReconnect(t *testing.T) {
	for _, provider := range []models.InboxProvider{models.InboxProviderGoogle, models.InboxProviderOutlook} {
		if q := authQuery(t, provider, ""); q.Has("login_hint") {
			t.Errorf("%s: a first connect must not preselect an account, got login_hint=%q", provider, q.Get("login_hint"))
		}
		q := authQuery(t, provider, "owner@example.com")
		if got := q.Get("login_hint"); got != "owner@example.com" {
			t.Errorf("%s: login_hint = %q, want the mailbox being renewed", provider, got)
		}
	}
}

// The approval link grants tenant-wide exactly what single-mailbox sign-in asks
// for, never .default, which would also carry the whole-organization app permissions.
func TestOutlookAdminApprovalURLCoversOnlyTheSigninScopes(t *testing.T) {
	cfg := config.OutlookOauth2Inbox("https://api.example.com")
	cfg.ClientID = "client"
	u, err := url.Parse(outlookAdminApprovalURL(cfg))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if u.Host != "login.microsoftonline.com" || u.Path != "/organizations/v2.0/adminconsent" {
		t.Fatalf("approval url = %s", u)
	}
	if q.Get("client_id") != "client" || q.Get("redirect_uri") != "https://api.example.com/addresses/outlook/callback" || q.Get("state") != OutlookAdminApprovalState {
		t.Fatalf("approval query = %v", q)
	}
	if q.Get("scope") != strings.Join(cfg.Scopes, " ") || strings.Contains(q.Get("scope"), ".default") {
		t.Fatalf("approval scope = %q", q.Get("scope"))
	}
	if got := outlookAdminApprovalURL(&oauth2.Config{Endpoint: oauth2.Endpoint{AuthURL: "https://provider.example.com/authorize"}}); got != "" {
		t.Fatalf("a non-Microsoft endpoint produced %q", got)
	}
}
