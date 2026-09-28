package delegation

import (
	"bytes"
	"context"
	"encoding/base64"
	"testing"
)

// The Directory API hands photoData back as web-safe base64, padded or not.
func TestDecodeWebSafe(t *testing.T) {
	raw := []byte{0xfb, 0xff, 0xfe, 0x01, 0x02}
	for _, enc := range []string{base64.URLEncoding.EncodeToString(raw), base64.RawURLEncoding.EncodeToString(raw)} {
		got, err := decodeWebSafe(enc)
		if err != nil || !bytes.Equal(got, raw) {
			t.Fatalf("decodeWebSafe(%q) = %v, %v", enc, got, err)
		}
	}
	if got, err := decodeWebSafe(""); got != nil || err != nil {
		t.Fatalf("empty photoData = %v, %v", got, err)
	}
}

// Graph answers 404 for a user with no photo, which is no photo rather than a failure.
func TestGraphPhotoNotFoundIsNone(t *testing.T) {
	s, p, _, _, _ := newTestService(t)
	ts := s.microsoftSource("11111111-1111-1111-1111-111111111111")
	got, err := s.getBytes(context.Background(), ts, p.URL+"/graph/users/nobody@contoso.com/photos/240x240/$value")
	if got != nil || err != nil {
		t.Fatalf("missing photo = %v, %v", got, err)
	}
}
