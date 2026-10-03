package config

import (
	"os"
	"strings"
)

// SlackSigningSecret verifies requests Slack sends to the events,
// interactivity and command URLs. Empty turns those endpoints off.
func SlackSigningSecret() string {
	return strings.TrimSpace(os.Getenv("SLACK_SIGNING_SECRET"))
}

// BackendPublicURL is the API's public origin as third parties call it:
// BACKEND_PUBLIC_URL, then API_PUBLIC_URL, then the local dev default.
func BackendPublicURL() string {
	for _, key := range []string{"BACKEND_PUBLIC_URL", "API_PUBLIC_URL"} {
		if v := strings.TrimRight(strings.TrimSpace(os.Getenv(key)), "/"); v != "" {
			return v
		}
	}
	return "http://localhost:8080"
}
