package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestLoggerKeepsCredentialsOutOfTheLog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var buf bytes.Buffer
	prev := gin.DefaultWriter
	gin.DefaultWriter = &buf
	t.Cleanup(func() { gin.DefaultWriter = prev })

	r := gin.New()
	r.Use(RequestLogger())
	ok := func(c *gin.Context) { c.Status(http.StatusNoContent) }
	r.POST("/api/v1/integrations/inbound/calendly/:secret", ok)
	r.POST("/unsubscribe/:token/resubscribe", ok)
	r.GET("/mailboxes/:remoteId/warmup-tokens/:token", ok)
	r.GET("/invitations/lookup", ok)

	cases := []struct{ method, target, want, leak string }{
		{"POST", "/api/v1/integrations/inbound/calendly/whsec-abc", "/api/v1/integrations/inbound/calendly/:secret", "whsec-abc"},
		{"POST", "/unsubscribe/unsub-tok/resubscribe", "/unsubscribe/:token/resubscribe", "unsub-tok"},
		{"GET", "/mailboxes/remote-7/warmup-tokens/warm-tok", "/mailboxes/remote-7/warmup-tokens/:token", "warm-tok"},
		{"GET", "/invitations/lookup?token=invite-tok", "/invitations/lookup", "invite-tok"},
	}
	for _, tc := range cases {
		buf.Reset()
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(tc.method, tc.target, nil))
		line := buf.String()
		if strings.Contains(line, tc.leak) {
			t.Errorf("%s logged the credential: %q", tc.target, line)
		}
		if !strings.Contains(line, `"`+tc.want+`"`) {
			t.Errorf("%s logged %q, want path %q", tc.target, line, tc.want)
		}
	}
}
