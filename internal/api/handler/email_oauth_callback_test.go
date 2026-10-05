package handler

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

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
