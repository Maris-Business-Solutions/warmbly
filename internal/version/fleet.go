package version

import (
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/mod/semver"
)

// CheckFleetTarget forbids automatically running code newer than its control plane.
func CheckFleetTarget(backend, target string) error {
	backend = FleetImageTag(backend)
	target = FleetImageTag(target)
	if backend == target && target != "" && target != "dev" {
		return nil
	}
	b := "v" + strings.TrimPrefix(backend, "v")
	t := "v" + strings.TrimPrefix(target, "v")
	if semver.IsValid(b) && semver.IsValid(t) && semver.Compare(t, b) <= 0 {
		return nil
	}
	return fmt.Errorf("fleet target %q is not compatible with backend %q; upgrade the backend first", target, backend)
}

// FleetImageTag maps main builds to the immutable SHA tag published by CI.
func FleetImageTag(v string) string {
	if sha, ok := strings.CutPrefix(v, "dev-"); ok && len(sha) == 40 {
		if _, err := hex.DecodeString(sha); err == nil {
			return sha
		}
	}
	return v
}
