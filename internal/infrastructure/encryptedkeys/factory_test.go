package encryptedkeys

import (
	"testing"
)

func TestFromEnv_UnknownProvider(t *testing.T) {
	t.Setenv("ENCRYPTED_KEYS_PROVIDER", "lol")
	if _, err := FromEnv(Deps{}, ""); err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestFromEnv_HTTPNeedsURLAndToken(t *testing.T) {
	t.Setenv("ENCRYPTED_KEYS_PROVIDER", "http")
	t.Setenv("ENCRYPTED_KEYS_BACKEND_URL", "")
	t.Setenv("ENCRYPTED_KEYS_WORKER_TOKEN", "")
	t.Setenv("NODE_BROKER_TOKEN", "")
	if _, err := FromEnv(Deps{}, ""); err == nil {
		t.Fatal("expected error when http config missing")
	}
}

func TestFromEnv_HTTPHappy(t *testing.T) {
	t.Setenv("ENCRYPTED_KEYS_PROVIDER", "http")
	t.Setenv("ENCRYPTED_KEYS_BACKEND_URL", "http://api.example.com")
	t.Setenv("ENCRYPTED_KEYS_WORKER_TOKEN", "tok")
	s, err := FromEnv(Deps{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if s.Name() != "http" {
		t.Fatalf("expected http, got %q", s.Name())
	}
}

func TestFromEnv_PostgresNeedsDB(t *testing.T) {
	t.Setenv("ENCRYPTED_KEYS_PROVIDER", "postgres")
	if _, err := FromEnv(Deps{DB: nil}, ""); err == nil {
		t.Fatal("expected error when DB nil")
	}
}

func TestFromEnv_DynamoNoLongerSupported(t *testing.T) {
	// The dynamodb DEK provider was removed; "dynamodb" must now be rejected
	// as an unknown provider rather than silently constructing a store.
	t.Setenv("ENCRYPTED_KEYS_PROVIDER", "dynamodb")
	if _, err := FromEnv(Deps{}, ""); err == nil {
		t.Fatal("expected error: dynamodb provider was removed")
	}
}

func TestFromEnv_FallbackUsedWhenUnset(t *testing.T) {
	t.Setenv("ENCRYPTED_KEYS_PROVIDER", "")
	t.Setenv("ENCRYPTED_KEYS_BACKEND_URL", "http://x")
	t.Setenv("ENCRYPTED_KEYS_WORKER_TOKEN", "t")
	s, err := FromEnv(Deps{}, "http")
	if err != nil {
		t.Fatal(err)
	}
	if s.Name() != "http" {
		t.Fatalf("expected fallback to apply, got %q", s.Name())
	}
}
