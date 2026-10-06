package handler

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/warmbly/warmbly/internal/api/middleware"
	"github.com/warmbly/warmbly/internal/infrastructure/storage"
)

type imageRoundTrip func(*http.Request) (*http.Response, error)

func (f imageRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDashboardImageFetch(t *testing.T) {
	image := pngBytes(t, 2, 2)
	cases := []struct {
		name, source, redirect string
		public                 bool
		status                 int
		body                   []byte
		length                 int64
		ok                     bool
	}{
		{name: "public PNG", source: "https://cdn.example/image", status: 200, body: image, ok: true},
		{name: "SVG", source: "https://cdn.example/image", status: 200, body: []byte(`<svg xmlns="http://www.w3.org/2000/svg"><rect width="20" height="20"/></svg>`), ok: true},
		{name: "HTML masquerading as image", source: "https://cdn.example/image", status: 200, body: []byte(`<html><script>alert(1)</script></html>`)},
		{name: "truncated SVG", source: "https://cdn.example/image", status: 200, body: []byte(`<svg xmlns="http://www.w3.org/2000/svg">`)},
		{name: "scripted SVG", source: "https://cdn.example/image", status: 200, body: []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)},
		{name: "SVG inline event", source: "https://cdn.example/image", status: 200, body: []byte(`<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"/>`)},
		{name: "SVG embedded HTML", source: "https://cdn.example/image", status: 200, body: []byte(`<svg xmlns="http://www.w3.org/2000/svg"><foreignObject/></svg>`)},
		{name: "multiple XML roots", source: "https://cdn.example/image", status: 200, body: []byte(`<svg xmlns="http://www.w3.org/2000/svg"/><html/>`)},
		{name: "XML external entity", source: "https://cdn.example/image", status: 200, body: []byte(`<!DOCTYPE svg SYSTEM "https://example.com/x"><svg xmlns="http://www.w3.org/2000/svg"/>`)},
		{name: "empty image", source: "https://cdn.example/image", status: 200},
		{name: "known oversized body", source: "https://cdn.example/image", status: 200, body: image, length: dashboardImageMaxBytes + 1},
		{name: "streamed oversized body", source: "https://cdn.example/image", status: 200, body: bytes.Repeat([]byte("x"), dashboardImageMaxBytes+1), length: -1},
		{name: "upstream error", source: "https://cdn.example/image", status: 404, body: image},
		{name: "plaintext refused", source: "http://cdn.example/image", status: 200, body: image},
		{name: "embedded credentials refused", source: "https://user:password@cdn.example/image", status: 200, body: image},
		{name: "non-web port refused", source: "https://cdn.example:6379/image", status: 200, body: image},
		{name: "anonymous arbitrary fetch refused", source: "https://cdn.example/image", public: true, status: 200, body: image},
		{name: "public Slack avatar", source: "https://avatars.slack-edge.com/profile.png", public: true, status: 200, body: image, ok: true},
		{name: "public gravatar", source: "https://secure.gravatar.com/avatar/abc", public: true, status: 200, body: image, ok: true},
		{name: "gravatar other paths refused", source: "https://secure.gravatar.com/redirect", public: true, status: 200, body: image},
		{name: "lookalike provider refused", source: "https://avatars.slack-edge.com.evil.example/profile.png", public: true, status: 200, body: image},
		{name: "public redirect cannot escape allowlist", source: "https://secure.gravatar.com/avatar/abc", public: true, status: 302, redirect: "https://cdn.example/image", body: image},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: imageRoundTrip(func(req *http.Request) (*http.Response, error) {
				calls++
				if req.Header.Get("Authorization") != "" || req.Header.Get("Cookie") != "" || req.Header.Get("Referer") != "" {
					t.Fatal("forwarded browser credentials")
				}
				if calls > 1 {
					t.Fatal("followed disallowed public redirect")
				}
				header := http.Header{"Content-Type": {"image/png"}}
				if tc.redirect != "" {
					header.Set("Location", tc.redirect)
				}
				return &http.Response{StatusCode: tc.status, Header: header, Body: io.NopCloser(bytes.NewReader(tc.body)), ContentLength: tc.length, Request: req}, nil
			})}
			data, contentType, err := loadDashboardImage(context.Background(), tc.source, nil, client, tc.public)
			if tc.ok {
				if err != nil || !bytes.Equal(data, tc.body) || !strings.HasPrefix(contentType, "image/") {
					t.Fatalf("image not preserved: type=%q err=%v", contentType, err)
				}
			} else if err == nil {
				t.Fatal("unsafe image accepted")
			}
		})
	}
}

func TestDashboardImageRequestLimits(t *testing.T) {
	r := gin.New()
	r.POST("/image", (&Handler{}).DashboardImage)
	for _, body := range []string{`{`, `{}`, `{"url":null}`, `{"url":42}`, `{"url":"https://example.com/\u0000"}`, `{"url":"https://example.com/` + strings.Repeat("a", 8192) + `"}`, strings.Repeat(" ", 20<<10) + `{"url":"https://example.com/"}`} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/image", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("invalid request returned %d", w.Code)
		}
	}
}

func TestDashboardImageOwnedStorage(t *testing.T) {
	ctx := context.Background()
	store, err := storage.NewFilesystem(t.TempDir(), "http://private-minio.example/public")
	if err != nil {
		t.Fatal(err)
	}
	source, err := store.PutPublic(ctx, "avatars/example.png", bytes.NewReader(pngBytes(t, 2, 2)), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: imageRoundTrip(func(*http.Request) (*http.Response, error) {
		t.Fatal("owned storage must not be fetched over HTTP")
		return nil, nil
	})}
	if _, _, err := loadDashboardImage(ctx, source, store, client, true); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"bodies/private.png", "avatars/../bodies/private.png", "avatars/missing.png"} {
		if _, _, err := loadDashboardImage(ctx, store.PublicURL(key), store, client, true); err == nil {
			t.Fatalf("invalid public key accepted: %s", key)
		}
	}
	h := &Handler{Storage: store}
	r := gin.New()
	r.POST("/public", h.PublicDashboardImage)
	r.POST("/authenticated", (&middleware.Handler{}).AuthMiddleware(), h.DashboardImage)
	for _, tc := range []struct {
		path string
		want int
	}{{"/public", http.StatusOK}, {"/authenticated", http.StatusUnauthorized}} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(`{"url":"`+source+`"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Fatalf("%s returned %d", tc.path, w.Code)
		}
		if w.Code == http.StatusOK && (w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(w.Header().Get("Content-Security-Policy"), "sandbox")) {
			t.Fatal("missing image response isolation headers")
		}
	}
}
