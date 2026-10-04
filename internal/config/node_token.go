package config

import (
	"os"
	"strings"
)

// NodeAPIToken is the credential a fleet node presents on the node-only
// internal routes: NODE_BROKER_TOKEN, else the shared internal token.
func NodeAPIToken() string {
	for _, key := range []string{"NODE_BROKER_TOKEN", "ENCRYPTED_KEYS_WORKER_TOKEN", "INTERNAL_API_TOKEN"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return ""
}
