package config

import "testing"

func TestGoogleMailboxOAuthUsesBoxCredentials(t *testing.T) {
	t.Setenv("BOX_GOOGLE_CLIENT_ID", "mailbox-client")
	t.Setenv("BOX_GOOGLE_CLIENT_SECRET", "mailbox-secret")
	t.Setenv("GOOGLE_CLIENT_ID", "sso-client")
	t.Setenv("GOOGLE_CLIENT_SECRET", "sso-secret")
	conf := GoogleOauth2Inbox("https://api.example.test")
	if conf.ClientID != "mailbox-client" || conf.ClientSecret != "mailbox-secret" {
		t.Fatal("mailbox OAuth must use BOX credentials, not dashboard SSO credentials")
	}
	if conf.RedirectURL != "https://api.example.test/addresses/google/callback" {
		t.Fatalf("unexpected mailbox callback: %s", conf.RedirectURL)
	}
}

func TestGoogleOAuthConnect(t *testing.T) {
	for _, tt := range []struct {
		name, flag, id, secret string
		want                   bool
	}{
		{"unconfigured", "", "", "", false},
		{"configured defaults on", "", "client", "secret", true},
		{"explicit enable", "true", "client", "secret", true},
		{"explicit disable", "false", "client", "secret", false},
		{"invalid fails closed", "invalid", "client", "secret", false},
		{"missing id", "true", "", "secret", false},
		{"missing secret", "true", "client", "", false},
		{"blank credentials", "true", " ", " ", false},
		{"trim flag", " true ", "client", "secret", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("BOX_GOOGLE_OAUTH_CONNECT", tt.flag)
			t.Setenv("BOX_GOOGLE_CLIENT_ID", tt.id)
			t.Setenv("BOX_GOOGLE_CLIENT_SECRET", tt.secret)
			if got := GoogleOAuthConnect(); got != tt.want {
				t.Fatalf("GoogleOAuthConnect() = %v, want %v", got, tt.want)
			}
		})
	}
}
