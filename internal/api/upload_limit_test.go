package api

import (
	"net/http/httptest"
	"testing"
)

// The contact file uploads cap their own bodies at 50 MB; every other route
// keeps the global 10 MB cap.
func TestLargeUploadRouteIsOnlyTheContactUploads(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		want         bool
	}{
		{"POST", "/v1/contacts/imports", true},
		{"POST", "/v1/contacts/import/preview", true},
		{"POST", "/v1/contacts/import/commit", true},
		{"GET", "/v1/contacts/imports", false},
		{"POST", "/v1/contacts/imports/abc/start", false},
		{"POST", "/v1/contacts", false},
		{"POST", "/v1/emails/imports", false},
	} {
		if got := largeUploadRoute(httptest.NewRequest(tc.method, tc.path, nil)); got != tc.want {
			t.Errorf("%s %s: got %v, want %v", tc.method, tc.path, got, tc.want)
		}
	}
}
