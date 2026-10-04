package fleetnode

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/warmbly/warmbly/internal/pkg/crypt"
	"github.com/warmbly/warmbly/internal/repository"
)

type memoryJoinToken struct {
	repository.FleetSettingsRepository
	hash      string
	expiresAt *time.Time
}

func (m *memoryJoinToken) GetJoinToken(context.Context) (string, *time.Time, error) {
	return m.hash, m.expiresAt, nil
}

func (m *memoryJoinToken) SetJoinToken(_ context.Context, hash string, expiresAt time.Time) error {
	m.hash, m.expiresAt = hash, &expiresAt
	return nil
}

// A join token works for any number of machines inside its window and for none after it.
func TestJoinTokenExpires(t *testing.T) {
	ctx := context.Background()
	store := &memoryJoinToken{}
	s := &Service{settings: store}

	tok, expiresAt, err := s.IssueJoinToken(ctx, 0)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if d := time.Until(expiresAt); d <= 0 || d > DefaultJoinTokenTTL {
		t.Fatalf("default expiry is %v away, want within %v", d, DefaultJoinTokenTTL)
	}
	for i := range 3 {
		if err := s.VerifyJoinToken(ctx, tok); err != nil {
			t.Fatalf("join %d inside the window: %v", i, err)
		}
	}
	if err := s.VerifyJoinToken(ctx, tok+"x"); !errors.Is(err, ErrBadToken) {
		t.Fatalf("wrong token: %v, want ErrBadToken", err)
	}

	past := time.Now().Add(-time.Second)
	store.expiresAt = &past
	if err := s.VerifyJoinToken(ctx, tok); !errors.Is(err, ErrJoinTokenExpired) {
		t.Fatalf("expired token: %v, want ErrJoinTokenExpired", err)
	}

	// A token stored before tokens carried an expiry is refused.
	store.expiresAt = nil
	if err := s.VerifyJoinToken(ctx, tok); !errors.Is(err, ErrJoinTokenExpired) {
		t.Fatalf("token with no expiry: %v, want ErrJoinTokenExpired", err)
	}
}

func TestJoinTokenTTLIsBounded(t *testing.T) {
	s := &Service{settings: &memoryJoinToken{}}
	if _, _, err := s.IssueJoinToken(context.Background(), MaxJoinTokenTTL+time.Hour); !errors.Is(err, ErrJoinTokenTTL) {
		t.Fatalf("over-long ttl: %v, want ErrJoinTokenTTL", err)
	}
}

// Only the hash is kept.
func TestJoinTokenStoresOnlyTheHash(t *testing.T) {
	store := &memoryJoinToken{}
	s := &Service{settings: store}
	tok, _, err := s.IssueJoinToken(context.Background(), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if store.hash != crypt.SHA256(tok) {
		t.Fatal("stored value is not the token's hash")
	}
}
