package handler

import (
	"net/http"
	"net/http/httptest"
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
