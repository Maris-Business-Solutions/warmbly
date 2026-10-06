package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func TestQueryValidationRejectsUnsupportedText(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, query := range []string{"q=%00", "q=valid&q=%00", "%00=value", "cursor=%00", "folder=%00", "tag=%00", "q=%FF", "%FF=value", "q=%ZZ", "q=unescaped;semicolon"} {
		t.Run(query, func(t *testing.T) {
			called := false
			r := gin.New()
			r.Use(SecurityHeaders(), RequestIDMiddleware(), cors.New(cors.Config{AllowOrigins: []string{"https://dashboard.example.com"}, AllowMethods: []string{http.MethodGet}}), QueryValidation())
			r.GET("/v1/campaigns", func(c *gin.Context) {
				called = true
				c.Status(http.StatusOK)
			})
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/v1/campaigns?"+query, nil)
			req.Header.Set("Origin", "https://dashboard.example.com")
			r.ServeHTTP(w, req)
			if called || w.Code != http.StatusBadRequest {
				t.Fatalf("called = %t, status = %d; want false, 400", called, w.Code)
			}
			if !strings.Contains(w.Body.String(), `"code":"bad_request"`) || !strings.Contains(w.Body.String(), `"request_id":"`) {
				t.Errorf("missing structured error or request ID: %s", w.Body.String())
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Error("invalid queries must retain cache protection")
			}
			if w.Header().Get("Access-Control-Allow-Origin") != "https://dashboard.example.com" {
				t.Error("validation errors must be readable by the configured dashboard origin")
			}
		})
	}
}

func TestQueryValidationPreservesValidQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, query := range []string{"", "q=security&limit=5", "q=%27%22%3Cscript%3E", "q=hello%3Bworld", "q=%E4%BD%A0%E5%A5%BD", "q=one&q=two"} {
		t.Run(query, func(t *testing.T) {
			r := gin.New()
			r.Use(QueryValidation())
			r.GET("/v1/emails", func(c *gin.Context) {
				if c.Request.URL.RawQuery != query {
					t.Errorf("query was changed to %q", c.Request.URL.RawQuery)
				}
				c.Status(http.StatusOK)
			})
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/v1/emails?"+query, nil))
			if w.Code != http.StatusOK {
				t.Errorf("valid query returned %d", w.Code)
			}
		})
	}
}
