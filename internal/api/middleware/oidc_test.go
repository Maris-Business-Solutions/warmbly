package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestOidcMiddlewareStopsTheChainOnRefusal(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := map[string]*OidcHandler{
		"no key set":  {AppEnv: "production", Audience: "https://api.example.com/webhook/email"},
		"no audience": {AppEnv: "production"},
	}
	for name, h := range cases {
		t.Run(name, func(t *testing.T) {
			ran := false
			r := gin.New()
			r.POST("/webhook/email", h.Middleware(), func(c *gin.Context) {
				ran = true
				c.Status(http.StatusNoContent)
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/webhook/email", nil)
			req.Header.Set("Authorization", "Bearer not-a-token")
			r.ServeHTTP(w, req)

			if ran {
				t.Fatal("handler ran after the middleware refused the request")
			}
			if w.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403", w.Code)
			}
		})
	}
}
