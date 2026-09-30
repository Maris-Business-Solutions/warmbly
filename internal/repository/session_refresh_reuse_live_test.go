package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// A refresh token presented after the session rotated past it has been used
// twice. Inside the race window it is two tabs refreshing at once and the
// session lives; outside it, or for anything older, the session ends.
func TestLiveRefreshReuseRevokesOnlyOutsideTheRace(t *testing.T) {
	handle, pool := liveContactDB(t)
	requireSchemaVersion(t, pool, 237)
	repo := NewTokenRepostory(handle)
	ctx := context.Background()

	user := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, first_name, last_name, email, password_hash)
		VALUES ($1, 'Refresh', 'Live', $2, 'x')`, user, "refresh-"+user.String()[:8]+"@test.local"); err != nil {
		t.Fatalf("user: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, user) })

	newSession := func(refreshNonce string) uuid.UUID {
		id := uuid.New()
		if _, err := pool.Exec(ctx, `INSERT INTO sessions (id, user_id, created_at, expires_at, access_nonce, refresh_nonce)
			VALUES ($1, $2, now(), now() + interval '1 day', 'a0', $3)`, id, user, refreshNonce); err != nil {
			t.Fatalf("session: %v", err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM sessions WHERE id = $1`, id) })
		return id
	}
	revoked := func(id uuid.UUID) bool {
		var at *time.Time
		if err := pool.QueryRow(ctx, `SELECT revoked_at FROM sessions WHERE id = $1`, id).Scan(&at); err != nil {
			t.Fatalf("read session: %v", err)
		}
		return at != nil
	}
	rotate := func(id uuid.UUID, from, to string, at time.Time) {
		if xerr := repo.RefreshToken(ctx, id, from, "a-"+to, to, at); xerr != nil {
			t.Fatalf("rotate: %v", xerr)
		}
	}
	now := time.Now()
	window := now.Add(-2 * time.Minute)

	t.Run("lost race keeps the session", func(t *testing.T) {
		id := newSession("r1")
		rotate(id, "r1", "r2", now)
		got, xerr := repo.RevokeOnRefreshReuse(ctx, id, "r1", window)
		if xerr != nil {
			t.Fatalf("revoke: %v", xerr)
		}
		if got || revoked(id) {
			t.Fatal("the token just rotated away from ended the session inside the race window")
		}
	})

	t.Run("replay after the window ends the session", func(t *testing.T) {
		id := newSession("r1")
		rotate(id, "r1", "r2", now.Add(-10*time.Minute))
		got, xerr := repo.RevokeOnRefreshReuse(ctx, id, "r1", window)
		if xerr != nil {
			t.Fatalf("revoke: %v", xerr)
		}
		if !got || !revoked(id) {
			t.Fatal("a refresh token replayed after the window left the session alive")
		}
	})

	t.Run("an older token ends the session at once", func(t *testing.T) {
		id := newSession("r1")
		rotate(id, "r1", "r2", now.Add(-time.Minute))
		rotate(id, "r2", "r3", now)
		got, xerr := repo.RevokeOnRefreshReuse(ctx, id, "r1", window)
		if xerr != nil {
			t.Fatalf("revoke: %v", xerr)
		}
		if !got || !revoked(id) {
			t.Fatal("a refresh token two rotations old left the session alive")
		}
	})

	t.Run("the current token never ends the session", func(t *testing.T) {
		id := newSession("r1")
		rotate(id, "r1", "r2", now.Add(-10*time.Minute))
		got, xerr := repo.RevokeOnRefreshReuse(ctx, id, "r2", window)
		if xerr != nil {
			t.Fatalf("revoke: %v", xerr)
		}
		if got || revoked(id) {
			t.Fatal("the current refresh token ended its own session")
		}
	})
}
