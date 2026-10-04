package handler

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/infrastructure/storage"
)

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(1, 1, color.RGBA{R: 200, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestReencodeLogoDropsTrailingBytes(t *testing.T) {
	src := append(pngBytes(t, 64, 64), []byte("<html><script>alert(1)</script></html>")...)
	out, mime, ext, xerr := reencodeLogo(src)
	if xerr != nil {
		t.Fatalf("reencode: %v", xerr)
	}
	if mime != "image/png" || ext != ".png" {
		t.Fatalf("mime %q ext %q", mime, ext)
	}
	if bytes.Contains(out, []byte("<script>")) {
		t.Fatal("re-encoded logo kept the appended payload")
	}
}

func TestReencodeLogoRefuses(t *testing.T) {
	if _, _, _, xerr := reencodeLogo(pngBytes(t, 16, 16)); xerr == nil {
		t.Fatal("a 16px logo was accepted")
	}
	if _, _, _, xerr := reencodeLogo([]byte("<svg xmlns='http://www.w3.org/2000/svg'/>")); xerr == nil {
		t.Fatal("an SVG was accepted")
	}
}

func TestCheckAppLogo(t *testing.T) {
	ctx := context.Background()
	store, err := storage.NewFilesystem(t.TempDir(), "https://api.example/public")
	if err != nil {
		t.Fatal(err)
	}
	org, other := uuid.New(), uuid.New()
	key, err := appLogoKey(org, ".png")
	if err != nil {
		t.Fatal(err)
	}
	url, err := store.PutPublic(ctx, key, bytes.NewReader(pngBytes(t, 64, 64)), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	otherKey, _ := appLogoKey(other, ".png")
	otherURL, _ := store.PutPublic(ctx, otherKey, bytes.NewReader(pngBytes(t, 64, 64)), "image/png")

	accept := map[string]string{
		"empty":            "",
		"own upload":       url,
		"unchanged legacy": "https://cdn.legacy.example/old.png",
	}
	for name, u := range accept {
		current := ""
		if name == "unchanged legacy" {
			current = u
		}
		if xerr := checkAppLogo(ctx, store, org, u, current); xerr != nil {
			t.Errorf("%s refused: %v", name, xerr)
		}
	}
	refuse := map[string]string{
		"outside url":         "https://tracker.example/pixel.png",
		"other workspace":     otherURL,
		"missing object":      store.PublicURL("oauth-app-logos/" + org.String() + "/nope.png"),
		"traversal":           store.PublicURL("oauth-app-logos/" + org.String() + "/../" + other.String() + "/x.png"),
		"nested path":         store.PublicURL("oauth-app-logos/" + org.String() + "/a/b.png"),
		"wrong extension":     strings.TrimSuffix(url, ".png") + ".svg",
		"lookalike host":      strings.Replace(url, "api.example", "api.example.evil", 1),
		"another public type": store.PublicURL("avatars/" + org.String() + ".png"),
	}
	for name, u := range refuse {
		if xerr := checkAppLogo(ctx, store, org, u, ""); xerr == nil {
			t.Errorf("%s accepted: %s", name, u)
		}
	}
}
