package argon2

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestCheckParams(t *testing.T) {
	own, err := Hash("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckParams(own); err != nil {
		t.Fatalf("a hash this package made was refused: %v", err)
	}
	b64 := func(n int) string { return base64.RawStdEncoding.EncodeToString(make([]byte, n)) }
	salt, key := b64(16), b64(32)
	for name, h := range map[string]string{
		"low memory":       "$argon2id$v=19$m=4096,t=3,p=2$" + salt + "$" + key,
		"few iterations":   "$argon2id$v=19$m=65536,t=1,p=2$" + salt + "$" + key,
		"one lane":         "$argon2id$v=19$m=65536,t=3,p=1$" + salt + "$" + key,
		"short key":        "$argon2id$v=19$m=65536,t=3,p=2$" + salt + "$" + b64(16),
		"short salt":       "$argon2id$v=19$m=65536,t=3,p=2$" + b64(8) + "$" + key,
		"not argon2id":     "$argon2i$v=19$m=65536,t=3,p=2$" + salt + "$" + key,
		"not a phc string": "plaintext",
	} {
		if err := CheckParams(h); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if err := CheckParams("$argon2id$v=19$m=131072,t=4,p=4$" + salt + "$" + key); err != nil {
		t.Errorf("stronger parameters refused: %v", err)
	}
}

func TestHashAndVerify(t *testing.T) {
	passwords := []string{
		"",
		"password123",
		"🚀🔥unicodeTest123",
	}

	for _, pw := range passwords {
		t.Run(pw, func(t *testing.T) {
			hash, err := Hash(pw)
			if err != nil {
				t.Fatalf("Hash() error: %v", err)
			}

			// Hash should not be empty
			if hash == "" {
				t.Fatal("Hash() returned empty string")
			}

			// Hash should start with $argon2id$
			if !strings.HasPrefix(hash, "$argon2id$") {
				t.Errorf("Hash() returned unexpected format: %s", hash)
			}

			// Verify correct password
			ok, err := Verify(pw, hash)
			if err != nil {
				t.Fatalf("Verify() error: %v", err)
			}
			if !ok {
				t.Errorf("Verify() failed for correct password")
			}

			// Verify wrong password
			ok, err = Verify(pw+"wrong", hash)
			if err != nil {
				t.Fatalf("Verify() error on wrong password: %v", err)
			}
			if ok {
				t.Errorf("Verify() succeeded for wrong password")
			}
		})
	}
}

func TestDecodeHashInvalidFormat(t *testing.T) {
	tests := []struct {
		name    string
		hash    string
		wantErr error
	}{
		{"empty string", "", ErrInvalidHash},
		{"wrong variant", "$argon2i$v=19$m=65536,t=3,p=2$abc$def", ErrIncompatibleVariant},
		{"wrong version", "$argon2id$v=16$m=65536,t=3,p=2$abc$def", ErrIncompatibleVersion},
		{"malformed params", "$argon2id$v=19$m=abc,t=3,p=2$abc$def", ErrInvalidHash},
		{"invalid base64 salt", "$argon2id$v=19$m=65536,t=3,p=2$!!!$def", ErrInvalidHash},
		{"invalid base64 key", "$argon2id$v=19$m=65536,t=3,p=2$abc$!!!", ErrInvalidHash},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, err := DecodeHash(tt.hash)
			if err == nil {
				t.Fatal("expected error but got nil")
			}
			if tt.wantErr != nil && err != tt.wantErr && err.Error() != tt.wantErr.Error() {
				t.Errorf("got error %v, want %v", err, tt.wantErr)
			}
		})
	}
}
