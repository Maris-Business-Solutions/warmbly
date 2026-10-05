package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/warmbly/warmbly/internal/app/email"
)

type returnOriginService struct {
	email.EmailService
	origin string
	state  string
}

func (s *returnOriginService) OAuthReturnOrigin(_ context.Context, state string) string {
	s.state = state
	return s.origin
}

func TestGoogleCallbackUsesStateBoundDashboard(t *testing.T) {
	t.Setenv("APP_URL", "https://app.warmbly.com")
	t.Setenv("APP_ORIGIN", "")
	t.Setenv("CORS_ALLOW_ORIGINS", "https://app.warmbly.com,https://tac-security-assessment.warmbly.com")
	for _, tt := range []struct{ name, origin, want string }{
		{"assessment", "https://tac-security-assessment.warmbly.com", "https://tac-security-assessment.warmbly.com"},
		{"primary dashboard", "https://app.warmbly.com", "https://app.warmbly.com"},
		{"legacy state", "", "https://app.warmbly.com"},
		{"untrusted origin", "https://evil.example.com", "https://app.warmbly.com"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc := &returnOriginService{origin: tt.origin}
			h := &Handler{EmailService: svc}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/addresses/google/callback?code=c&state=w.nonce", nil)
			h.EmailOAuthCallbackGmail(c)
			body := w.Body.String()
			if w.Code != http.StatusOK || svc.state != "w.nonce" || !strings.Contains(body, `var origin = "`+tt.want+`"`) || !strings.Contains(body, `var relay = "`+tt.want+`/oauth-return"`) {
				t.Fatalf("callback did not use the expected state-bound target: %d %s", w.Code, body)
			}
			if w.Header().Get("Referrer-Policy") != "no-referrer" || w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("callback must not leak through referrers or caches")
			}
		})
	}
	// Microsoft remains on its existing primary-dashboard routing.
	svc := &returnOriginService{origin: "https://tac-security-assessment.warmbly.com"}
	w := callbackRecorder(t, &Handler{EmailService: svc}, "code=c&state=w.nonce")
	if svc.state != "" || !strings.Contains(w.Body.String(), `var relay = "https://app.warmbly.com/oauth-return"`) {
		t.Fatal("Google callback routing must not change Microsoft OAuth")
	}
}

func callbackRecorder(t *testing.T, h *Handler, query string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/addresses/outlook/callback?"+query, nil)
	h.EmailOAuthCallbackOutlook(c)
	return w
}

// The approval link's return hands nothing to an opener and never falls back to the app scheme.
func TestOutlookAdminApprovalReturnIsAStandalonePage(t *testing.T) {
	h := &Handler{}
	ok := callbackRecorder(t, h, "admin_consent=True&tenant=11111111-1111-1111-1111-111111111111&state=oac_approval")
	if ok.Code != http.StatusOK || !strings.Contains(ok.Body.String(), "Approved") || strings.Contains(ok.Body.String(), "postMessage") || strings.Contains(ok.Body.String(), "warmbly://") {
		t.Fatalf("approved page = %d %s", ok.Code, ok.Body.String())
	}
	refused := callbackRecorder(t, h, "error=access_denied&state=oac_approval")
	if !strings.Contains(refused.Body.String(), "Not approved") {
		t.Fatalf("refused page = %s", refused.Body.String())
	}
}

var webFlag = regexp.MustCompile(`var web =\s*true\s*;`)

// A dashboard sign-in without an opener returns to the dashboard; the native app's still gets its scheme.
func TestCallbackReturnsDashboardFlowsToTheDashboard(t *testing.T) {
	t.Setenv("APP_ORIGIN", "")
	t.Setenv("APP_URL", "https://app.acme.io/")
	h := &Handler{}
	for _, state := range []string{"w.abc", "gac_abc", "mac_abc"} {
		w := callbackRecorder(t, h, "code=c&state="+state)
		body := w.Body.String()
		if !strings.Contains(body, `var relay = "https://app.acme.io/oauth-return"`) || !webFlag.MatchString(body) {
			t.Fatalf("state %s page = %s", state, body)
		}
		if w.Header().Get("Referrer-Policy") != "no-referrer" {
			t.Fatalf("state %s referrer policy = %q", state, w.Header().Get("Referrer-Policy"))
		}
	}
	native := callbackRecorder(t, h, "code=c&state=abc").Body.String()
	if webFlag.MatchString(native) || !strings.Contains(native, "warmbly://email-oauth") {
		t.Fatalf("native page = %s", native)
	}
}

func TestIntegrationCallbackReturnsToTheDashboardWithoutAnOpener(t *testing.T) {
	t.Setenv("APP_ORIGIN", "")
	t.Setenv("APP_URL", "https://app.acme.io")
	body := serve(t, (&Handler{}).IntegrationOAuthCallback, "/cb?state=s&code=c").Body.String()
	if !strings.Contains(body, `var relay = "https://app.acme.io/oauth-return"`) || !strings.Contains(body, "source=integration") {
		t.Fatalf("integration page = %s", body)
	}
}
