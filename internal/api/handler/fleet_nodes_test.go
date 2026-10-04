package handler

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/models"
)

func TestHeartbeatAddressPrefersBackendObservedPublicIPv4(t *testing.T) {
	tests := []struct {
		name     string
		reported string
		observed string
		want     string
	}{
		{"observed public address replaces stale report", "198.51.100.8", "8.8.8.8", "8.8.8.8"},
		{"private proxy hop keeps reported public address", "1.1.1.1", "10.0.0.4", "1.1.1.1"},
		{"carrier grade nat is not a public address", "1.1.1.1", "100.64.2.3", "1.1.1.1"},
		{"mapped public IPv4 is canonicalized", "1.1.1.1", "::ffff:8.8.8.8", "8.8.8.8"},
		{"private direct address is still useful", "", "10.0.0.4", "10.0.0.4"},
		{"public IPv6 does not replace requested IPv4", "1.1.1.1", "2001:4860:4860::8888", "1.1.1.1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := heartbeatAddress(tc.reported, tc.observed); got != tc.want {
				t.Fatalf("heartbeatAddress(%q, %q) = %q, want %q", tc.reported, tc.observed, got, tc.want)
			}
		})
	}
}

// envLines turns a rendered node env into a lookup, so assertions name a
// setting rather than a line number.
func envLines(t *testing.T, rendered string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, line := range strings.Split(rendered, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			t.Fatalf("line is not KEY=value: %q", line)
		}
		out[k] = v
	}
	return out
}

// setInstanceEnv puts the process in the shape of an AWS-backed control plane.
func setInstanceEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ENCRYPTED_KEYS_BACKEND_URL", "https://api.example.com/")
	t.Setenv("INTERNAL_API_TOKEN", "tok")
	t.Setenv("KMS_PROVIDER", "aws")
	t.Setenv("KMS_AWS_KEY_ID", "alias/warmbly")
	t.Setenv("BLOB_PROVIDER", "s3")
	t.Setenv("BLOB_BUCKET", "warmbly-blobs")
	t.Setenv("AWS_REGION", "eu-central-1")
	t.Setenv("CREDENTIALS_ENCRYPTION_KEY", "deadbeef")
	t.Setenv("NATS_URL", "tls://bus.example.com:4222")
	t.Setenv("REDIS", "rediss://bus.example.com:6380")
}

// A node reports its own crashes, and the only place it can learn where to send
// them is the env the join endpoint renders. Both backends travel: an instance
// on PostHog and an instance on Sentry each get a fleet that reports.
func TestRenderNodeEnvCarriesErrorTracking(t *testing.T) {
	t.Setenv("POSTHOG_KEY", "phc_example")
	t.Setenv("POSTHOG_HOST", "https://eu.i.posthog.com")
	t.Setenv("POSTHOG_ERROR_TRACKING", "false")
	t.Setenv("SENTRY_DSN", "https://k@example.invalid/1")

	env := envLines(t, renderNodeEnv(uuid.New(), models.NodeRoleWorker, ""))

	for key, want := range map[string]string{
		"POSTHOG_KEY":            "phc_example",
		"POSTHOG_HOST":           "https://eu.i.posthog.com",
		"POSTHOG_ERROR_TRACKING": "false",
		"SENTRY_DSN":             "https://k@example.invalid/1",
	} {
		if env[key] != want {
			t.Errorf("%s = %q, want %q", key, env[key], want)
		}
	}
}

// A node has no cloud credential and no way to be given one, so a provider
// that needs a credential has to arrive translated. Shipping "aws" here is how
// a joined node ends up unable to open a single mailbox.
func TestRenderNodeEnvBrokersCredentialProviders(t *testing.T) {
	setInstanceEnv(t)
	env := envLines(t, renderNodeEnv(uuid.New(), models.NodeRoleWorker, "eu-central"))

	if env["KMS_PROVIDER"] != "brokered" {
		t.Errorf("KMS_PROVIDER = %q, want brokered", env["KMS_PROVIDER"])
	}
	if env["BLOB_PROVIDER"] != "brokered" {
		t.Errorf("BLOB_PROVIDER = %q, want brokered", env["BLOB_PROVIDER"])
	}
	if _, ok := env["AWS_ACCESS_KEY_ID"]; ok {
		t.Error("a node was handed an AWS credential")
	}
}

// A provider that needs no credential works on a node as it stands, and
// translating it would break a local install for nothing.
func TestRenderNodeEnvPassesThroughLocalProviders(t *testing.T) {
	setInstanceEnv(t)
	t.Setenv("KMS_PROVIDER", "local")
	t.Setenv("BLOB_PROVIDER", "filesystem")
	t.Setenv("BLOB_FS_ROOT", "/data/blobs")

	env := envLines(t, renderNodeEnv(uuid.New(), models.NodeRoleWorker, ""))
	if env["KMS_PROVIDER"] != "local" {
		t.Errorf("KMS_PROVIDER = %q, want local", env["KMS_PROVIDER"])
	}
	if env["BLOB_PROVIDER"] != "filesystem" {
		t.Errorf("BLOB_PROVIDER = %q, want filesystem", env["BLOB_PROVIDER"])
	}
	if env["BLOB_FS_ROOT"] != "/data/blobs" {
		t.Errorf("BLOB_FS_ROOT = %q", env["BLOB_FS_ROOT"])
	}
}

// The regression this file exists for: every name sent has to be one the
// node's own code reads. S3_BUCKET and KMS_KEY_ID were read by nothing, so a
// node fell back to the default bucket and the default key alias in silence.
func TestRenderNodeEnvSendsNamesTheNodeReads(t *testing.T) {
	setInstanceEnv(t)
	t.Setenv("S3_BUCKET", "should-not-travel")
	t.Setenv("KMS_KEY_ID", "should-not-travel")

	env := envLines(t, renderNodeEnv(uuid.New(), models.NodeRoleWorker, ""))
	for _, dead := range []string{"S3_BUCKET", "KMS_KEY_ID"} {
		if _, ok := env[dead]; ok {
			t.Errorf("%s is still sent; nothing reads it", dead)
		}
	}
	if env["BLOB_BUCKET"] != "warmbly-blobs" {
		t.Errorf("BLOB_BUCKET = %q, want warmbly-blobs", env["BLOB_BUCKET"])
	}
	if env["KMS_AWS_KEY_ID"] != "alias/warmbly" {
		t.Errorf("KMS_AWS_KEY_ID = %q, want alias/warmbly", env["KMS_AWS_KEY_ID"])
	}
}

// A worker reaches relational data through the internal API and nothing else.
// The DSN must not travel even when the control plane has one to send.
func TestRenderNodeEnvWithholdsDSNFromWorker(t *testing.T) {
	setInstanceEnv(t)
	t.Setenv("PRIMARY_DB", "postgres://u:p@db/warmbly")

	env := envLines(t, renderNodeEnv(uuid.New(), models.NodeRoleWorker, ""))
	if _, ok := env["PRIMARY_DB"]; ok {
		t.Fatal("a worker was handed a database DSN")
	}
	if env["ENCRYPTED_KEYS_PROVIDER"] != "http" {
		t.Errorf("ENCRYPTED_KEYS_PROVIDER = %q, want http", env["ENCRYPTED_KEYS_PROVIDER"])
	}
}

// A consumer is control plane: it opens Postgres itself and cannot boot
// without the DSN, which is why joining one used to produce a node that died
// on its first start.
func TestRenderNodeEnvGivesConsumerTheDSN(t *testing.T) {
	setInstanceEnv(t)
	t.Setenv("PRIMARY_DB", "postgres://u:p@db/warmbly")

	id := uuid.New()
	env := envLines(t, renderNodeEnv(id, models.NodeRoleConsumer, ""))
	if env["PRIMARY_DB"] != "postgres://u:p@db/warmbly" {
		t.Errorf("PRIMARY_DB = %q", env["PRIMARY_DB"])
	}
	if env["ENCRYPTED_KEYS_PROVIDER"] != "postgres" {
		t.Errorf("ENCRYPTED_KEYS_PROVIDER = %q, want postgres", env["ENCRYPTED_KEYS_PROVIDER"])
	}
	// Only a worker claims a placement identity.
	if _, ok := env["WORKER_ID"]; ok {
		t.Error("a consumer was given a WORKER_ID")
	}
	if env["WARMBLY_NODE_ID"] != id.String() {
		t.Errorf("WARMBLY_NODE_ID = %q, want %s", env["WARMBLY_NODE_ID"], id)
	}
}

// An instance holding its DSN in SSM has none to send. The node still has to
// be able to fetch keys, so it falls back to the HTTP path rather than being
// left with a provider it cannot satisfy.
func TestRenderNodeEnvConsumerWithoutDSNFallsBackToHTTP(t *testing.T) {
	setInstanceEnv(t)
	t.Setenv("PRIMARY_DB", "")

	env := envLines(t, renderNodeEnv(uuid.New(), models.NodeRoleConsumer, ""))
	if _, ok := env["PRIMARY_DB"]; ok {
		t.Error("PRIMARY_DB was sent as an empty value")
	}
	if env["ENCRYPTED_KEYS_PROVIDER"] != "http" {
		t.Errorf("ENCRYPTED_KEYS_PROVIDER = %q, want http", env["ENCRYPTED_KEYS_PROVIDER"])
	}
}

// The worker's identity and the node row have to be the same machine.
func TestRenderNodeEnvWorkerIdentity(t *testing.T) {
	setInstanceEnv(t)
	id := uuid.New()
	env := envLines(t, renderNodeEnv(id, models.NodeRoleWorker, "eu-central"))

	if env["WORKER_ID"] != id.String() {
		t.Errorf("WORKER_ID = %q, want %s", env["WORKER_ID"], id)
	}
	if env["WARMBLY_NODE_REGION"] != "eu-central" {
		t.Errorf("WARMBLY_NODE_REGION = %q", env["WARMBLY_NODE_REGION"])
	}
	// The trailing slash on the instance URL must not survive into a base URL
	// the node concatenates paths onto.
	if env["ENCRYPTED_KEYS_BACKEND_URL"] != "https://api.example.com" {
		t.Errorf("ENCRYPTED_KEYS_BACKEND_URL = %q", env["ENCRYPTED_KEYS_BACKEND_URL"])
	}
}

// A consumer refreshes integration tokens and drains the CRM outbox, so it
// needs the integration client credentials; a worker never sees them.
func TestRenderNodeEnvIntegrationCredentialsReachConsumerOnly(t *testing.T) {
	setInstanceEnv(t)
	t.Setenv("HUBSPOT_OAUTH_CLIENT_ID", "hs-id")
	t.Setenv("HUBSPOT_OAUTH_CLIENT_SECRET", "hs-secret")

	consumer := envLines(t, renderNodeEnv(uuid.New(), models.NodeRoleConsumer, ""))
	if consumer["HUBSPOT_OAUTH_CLIENT_SECRET"] != "hs-secret" || consumer["HUBSPOT_OAUTH_CLIENT_ID"] != "hs-id" {
		t.Errorf("consumer missing HubSpot credentials: %v", consumer)
	}
	worker := envLines(t, renderNodeEnv(uuid.New(), models.NodeRoleWorker, ""))
	if _, ok := worker["HUBSPOT_OAUTH_CLIENT_SECRET"]; ok {
		t.Error("a worker was handed an integration client secret")
	}
}
