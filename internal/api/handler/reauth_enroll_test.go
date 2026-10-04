package handler

import (
	"testing"
	"time"

	"github.com/warmbly/warmbly/internal/app/token"
	"github.com/warmbly/warmbly/internal/models"
)

func TestEnrollmentGate(t *testing.T) {
	now := time.Now()
	recent := now.Add(-time.Minute)
	stale := now.Add(-token.ReauthWindow - time.Minute)

	cases := []struct {
		name      string
		session   models.Session
		hasFactor bool
		wantCode  string
	}{
		{"recent reauth passes", models.Session{CreatedAt: stale, ReauthAt: &recent}, true, ""},
		{"factor without reauth must confirm", models.Session{CreatedAt: recent}, true, "reauth_required"},
		{"stale reauth must confirm", models.Session{CreatedAt: stale, ReauthAt: &stale}, true, "reauth_required"},
		{"no factor, fresh sign-in passes", models.Session{CreatedAt: recent}, false, ""},
		{"no factor, old sign-in is refused", models.Session{CreatedAt: stale}, false, "reauth_no_factor"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sess := tc.session
			xerr := enrollmentGate(&sess, now, func() bool { return tc.hasFactor })
			got := ""
			if xerr != nil {
				got = xerr.Identifier
			}
			if got != tc.wantCode {
				t.Fatalf("got %q, want %q", got, tc.wantCode)
			}
		})
	}
}
