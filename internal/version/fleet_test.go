package version

import "testing"

func TestCheckFleetTarget(t *testing.T) {
	for _, tc := range []struct {
		backend, target string
		allowed         bool
	}{
		{"v0.6.31", "v0.6.31", true},
		{"0.6.31", "v0.6.30", true},
		{"v0.6.31", "v0.6.32", false},
		{"v0.6.31", "v1.0.0", false},
		{"v0.6.31-dev.1", "v0.6.31", false},
		{"v0.6.31-dev.2", "v0.6.31-dev.1", true},
		{"dev", "v0.6.31", false},
		{"", "v0.6.31", false},
		{"dev", "dev", false},
		{"fork-build", "fork-build", true},
		{"fork-build", "other-build", false},
		{"dev-0123456789abcdef0123456789abcdef01234567", "0123456789abcdef0123456789abcdef01234567", true},
		{"dev-0123456789abcdef0123456789abcdef01234567", "v0.6.31", false},
	} {
		if err := CheckFleetTarget(tc.backend, tc.target); (err == nil) != tc.allowed {
			t.Errorf("backend %q target %q: err=%v, allowed=%t", tc.backend, tc.target, err, tc.allowed)
		}
	}
}

func TestFleetImageTag(t *testing.T) {
	for _, v := range []string{"v0.6.31", "dev", "fork-build", "dev-invalid"} {
		if got := FleetImageTag(v); got != v {
			t.Fatalf("tag %q became %q", v, got)
		}
	}
	sha := "0123456789abcdef0123456789abcdef01234567"
	if got := FleetImageTag("dev-" + sha); got != sha {
		t.Fatalf("dev image tag = %q, want %q", got, sha)
	}
}
