package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/models"
)

// RequireKeyHolder leaves sessions and OAuth to the role gate and refuses a key it cannot check.
func TestRequireKeyHolderFailsClosed(t *testing.T) {
	h := &Handler{}
	for _, tc := range []struct {
		authType string
		want     int
	}{{AuthTypeJWT, http.StatusOK}, {AuthTypeOAuth, http.StatusOK}, {AuthTypeAPIKey, http.StatusForbidden}} {
		r := gin.New()
		r.Use(func(c *gin.Context) {
			c.Set(AuthTypeKey, tc.authType)
			c.Set(UserIDKey, uuid.NewString())
			c.Next()
		})
		r.GET("/x", h.RequireKeyHolder(models.PermManageSettings), func(c *gin.Context) { c.Status(http.StatusOK) })
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil))
		if w.Code != tc.want {
			t.Errorf("%s: status = %d, want %d", tc.authType, w.Code, tc.want)
		}
	}
}
