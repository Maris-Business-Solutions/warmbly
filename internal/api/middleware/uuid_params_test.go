package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestUUIDParams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	id := uuid.NewString()
	for _, resource := range []string{"emails", "campaigns", "contacts"} {
		for _, raw := range []string{"bounces", "templates", "import", "None", "cro-c6", id, strings.ToUpper(id), strings.ReplaceAll(id, "-", "")} {
			t.Run(resource+"/"+raw, func(t *testing.T) {
				called := false
				r := gin.New()
				r.Use(SecurityHeaders(), RequestIDMiddleware())
				group := r.Group("/"+resource, UUIDParams("id"))
				group.GET("/:id", func(c *gin.Context) {
					called = true
					if c.Param("id") != id {
						t.Errorf("id = %q; want %q", c.Param("id"), id)
					}
					c.Status(http.StatusNotFound)
				})
				w := httptest.NewRecorder()
				r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/"+resource+"/"+raw, nil))
				_, parseErr := uuid.Parse(raw)
				if parseErr == nil {
					if !called || w.Code != http.StatusNotFound {
						t.Fatal("valid UUID must reach the scoped handler")
					}
				} else if called || w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), `"code":"bad_request"`) || !strings.Contains(w.Body.String(), `"request_id":"`) {
					t.Fatalf("called=%t status=%d body=%s", called, w.Code, w.Body.String())
				}
			})
		}
	}
}

func TestUUIDParamsPreservesStaticRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/contacts", UUIDParams("id"))
	g.GET("/verification", func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/contacts/verification", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("static route status=%d", w.Code)
	}
}
