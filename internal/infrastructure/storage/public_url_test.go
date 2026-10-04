package storage

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// PublicURL must name exactly what PutPublic hands out, on every backend, or a
// caller checking "is this one of our public objects" refuses its own uploads.
func TestPublicURLMatchesPutPublic(t *testing.T) {
	ctx := context.Background()
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	for _, base := range []string{"", "https://cdn.example.com/public/"} {
		c, closeSrv := testClient(t, "logos", ok)
		c.PublicBaseURL = base
		url, err := c.PutPublic(ctx, "oauth-app-logos/a/b.png", strings.NewReader("x"), "image/png")
		closeSrv()
		if err != nil {
			t.Fatalf("s3 PutPublic (base %q): %v", base, err)
		}
		if got := c.PublicURL("oauth-app-logos/a/b.png"); got != url {
			t.Fatalf("s3 PublicURL (base %q) = %q, PutPublic returned %q", base, got, url)
		}
	}

	fs, err := NewFilesystem(t.TempDir(), "https://api.example.com/public")
	if err != nil {
		t.Fatal(err)
	}
	url, err := fs.PutPublic(ctx, "oauth-app-logos/a/b.png", strings.NewReader("x"), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if got := fs.PublicURL("oauth-app-logos/a/b.png"); got != url {
		t.Fatalf("filesystem PublicURL = %q, PutPublic returned %q", got, url)
	}
}
