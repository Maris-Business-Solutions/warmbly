package email

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/config"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/infrastructure/cache"
	"github.com/warmbly/warmbly/internal/models"
	"golang.org/x/oauth2"
)

func TestOAuthStartRejectsUntrustedReturnOrigin(t *testing.T) {
	t.Setenv("APP_URL", "https://app.example.com")
	t.Setenv("APP_ORIGIN", "")
	t.Setenv("CORS_ALLOW_ORIGINS", "https://assessment.example.com")
	svc := &emailService{}
	org := uuid.New()
	for _, tt := range []struct {
		origin string
		web    bool
	}{
		{"https://evil.example.com", true},
		{"https://assessment.example.com/path", true},
		{"https://assessment.example.com", false},
	} {
		if _, xerr := svc.OAuthStart(context.Background(), uuid.NewString(), &org, models.InboxProviderGoogle, "", tt.web, tt.origin); xerr != errx.ErrEmailOnboardReturnOrigin {
			t.Fatalf("untrusted return origin should fail before storing state, got %v", xerr)
		}
	}
}

func TestOAuthReauthRejectsUntrustedGoogleReturnOrigin(t *testing.T) {
	t.Setenv("APP_URL", "https://app.example.com")
	t.Setenv("APP_ORIGIN", "")
	t.Setenv("CORS_ALLOW_ORIGINS", "https://assessment.example.com")
	svc, repo, _, _ := reauthFixture("gmail", "owner@example.com")
	_, xerr := svc.OAuthReauth(context.Background(), repo.account.UserID, repo.account.OrganizationID, repo.account.ID, "https://evil.example.com")
	if xerr != errx.ErrEmailOnboardReturnOrigin {
		t.Fatalf("untrusted Google reauthorization origin should be refused, got %v", xerr)
	}
}

func TestOAuthReauthIgnoresMicrosoftReturnOrigin(t *testing.T) {
	svc, repo, _, _ := reauthFixture("outlook", "owner@example.com")
	_, xerr := svc.OAuthReauth(context.Background(), repo.account.UserID, repo.account.OrganizationID, repo.account.ID, "https://evil.example.com")
	if xerr != errx.ErrEmailOnboardOutlookNotConfigured {
		t.Fatalf("Microsoft must keep its existing routing and reach the credential check, got %v", xerr)
	}
}

func TestGoogleOAuthOriginStaysBoundToSingleUseState(t *testing.T) {
	redisURL := os.Getenv("WARMBLY_TEST_REDIS")
	if redisURL == "" {
		t.Skip("WARMBLY_TEST_REDIS not set")
	}
	c, err := cache.New(redisURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	t.Setenv("APP_URL", "https://app.example.com")
	t.Setenv("APP_ORIGIN", "")
	t.Setenv("CORS_ALLOW_ORIGINS", "https://assessment.example.com")
	t.Setenv("BOX_GOOGLE_CLIENT_ID", "client")
	t.Setenv("BOX_GOOGLE_CLIENT_SECRET", "secret")
	t.Setenv("BOX_GOOGLE_OAUTH_CONNECT", "")
	svc, repo, _, _ := reauthFixture("gmail", "owner@example.com")
	svc.r = c
	svc.oauthInbox = &config.Oauth2Inbox{Google: &oauth2.Config{ClientID: "client", ClientSecret: "secret", Endpoint: oauth2.Endpoint{AuthURL: "https://accounts.google.com/o/oauth2/auth"}}}
	ctx := context.Background()
	for _, reauth := range []bool{false, true} {
		var resp *models.EmailOnboardingStartResponse
		var xerr *errx.Error
		if reauth {
			t.Setenv("BOX_GOOGLE_OAUTH_CONNECT", "false")
			resp, xerr = svc.OAuthReauth(ctx, repo.account.UserID, repo.account.OrganizationID, repo.account.ID, "https://assessment.example.com")
		} else {
			resp, xerr = svc.OAuthStart(ctx, repo.account.UserID, repo.account.OrganizationID, models.InboxProviderGoogle, "", true, "https://assessment.example.com")
		}
		if xerr != nil {
			t.Fatal(xerr)
		}
		t.Cleanup(func() { _ = c.Del(ctx, onboardingStateKey(resp.State)).Err() })
		if origin := svc.OAuthReturnOrigin(ctx, resp.State); origin != "https://assessment.example.com" {
			t.Fatalf("state-bound origin = %q", origin)
		}
		if svc.OAuthReturnOrigin(ctx, "w.unknown") != "" || svc.OAuthReturnOrigin(ctx, "native") != "" {
			t.Fatal("unknown or native state must not select a dashboard")
		}
		sess, xerr := svc.takeOnboardingState(ctx, resp.State)
		if xerr != nil || sess.ReturnOrigin != "https://assessment.example.com" || sess.CodeVerifier == "" || sess.UserID != repo.account.UserID || (sess.EmailAccountID != nil) != reauth {
			t.Fatalf("callback metadata lookup must preserve the finish state: %+v, %v", sess, xerr)
		}
		if svc.OAuthReturnOrigin(ctx, resp.State) != "" {
			t.Fatal("consumed state must not select a dashboard")
		}
		if _, xerr := svc.takeOnboardingState(ctx, resp.State); xerr != errx.ErrEmailOnboardState {
			t.Fatal("OAuth finish state must remain single-use")
		}
	}
}
