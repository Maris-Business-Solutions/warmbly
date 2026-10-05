package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

// heartbeatOutcome sends one heartbeat with bearer and reports the status and whether it was update-only.
func heartbeatOutcome(t *testing.T, internal, broker, accept, bearer string) (int, bool) {
	t.Helper()
	t.Setenv("INTERNAL_API_TOKEN", internal)
	t.Setenv("NODE_BROKER_TOKEN", broker)
	t.Setenv("NODE_ACCEPT_INTERNAL_TOKEN", accept)
	brokerTokenOnce = sync.Once{}
	brokerToken = nil
	updateOnly := false
	h := &Handler{}
	r := gin.New()
	r.POST("/fleet/heartbeat", h.NodeHeartbeatAuthMiddleware(), func(c *gin.Context) {
		updateOnly = IsNodeUpdateOnly(c)
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodPost, "/fleet/heartbeat", nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code, updateOnly
}

func TestNodeHeartbeatAuth(t *testing.T) {
	for _, tc := range []struct {
		name                            string
		internal, broker, accept, token string
		code                            int
		updateOnly                      bool
	}{
		{"broker token is a full beat", "shared", "broker", "", "broker", http.StatusOK, false},
		{"shared token is update-only", "shared", "broker", "", "shared", http.StatusOK, true},
		{"shared token is a full beat while accepted", "shared", "broker", "true", "shared", http.StatusOK, false},
		{"single-token instance is a full beat", "shared", "", "", "shared", http.StatusOK, false},
		{"same value for both is a full beat", "same", "same", "", "same", http.StatusOK, false},
		{"wrong token is refused", "shared", "broker", "", "other", http.StatusUnauthorized, false},
		{"no token is refused", "shared", "broker", "", "", http.StatusUnauthorized, false},
		{"nothing configured is refused", "", "", "", "anything", http.StatusUnauthorized, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, updateOnly := heartbeatOutcome(t, tc.internal, tc.broker, tc.accept, tc.token)
			if code != tc.code || updateOnly != tc.updateOnly {
				t.Fatalf("got %d update-only=%v, want %d update-only=%v", code, updateOnly, tc.code, tc.updateOnly)
			}
		})
	}
}
