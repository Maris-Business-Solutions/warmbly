package cache

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

// testCache connects to WARMBLY_TEST_REDIS (for example redis://localhost:16379/15).
func testCache(t *testing.T) *Cache {
	t.Helper()
	url := os.Getenv("WARMBLY_TEST_REDIS")
	if url == "" {
		t.Skip("WARMBLY_TEST_REDIS not set")
	}
	c, err := New(url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// Parallel guesses cannot all pass: exactly limit of them are admitted.
func TestReserveAttemptAdmitsExactlyLimitUnderConcurrency(t *testing.T) {
	c := testCache(t)
	ctx := context.Background()
	key := "test_attempts:" + uuid.NewString()
	t.Cleanup(func() { _ = c.Del(ctx, key).Err() })

	const limit, callers = 5, 64
	var admitted atomic.Int64
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := c.ReserveAttempt(ctx, key, limit, time.Minute)
			if err != nil {
				t.Errorf("reserve: %v", err)
				return
			}
			if ok {
				admitted.Add(1)
			}
		}()
	}
	wg.Wait()
	if got := admitted.Load(); got != limit {
		t.Fatalf("admitted %d of %d parallel attempts, want %d", got, callers, limit)
	}
	if ttl := c.PTTL(ctx, key).Val(); ttl <= 0 {
		t.Fatalf("counter has no expiry: %v", ttl)
	}
}

// A refunded attempt gives its slot back, and a counter refunded to zero is gone.
func TestReleaseAttemptRefundsAndCleansUp(t *testing.T) {
	c := testCache(t)
	ctx := context.Background()
	key := "test_attempts:" + uuid.NewString()
	t.Cleanup(func() { _ = c.Del(ctx, key).Err() })

	if ok, _ := c.ReserveAttempt(ctx, key, 1, time.Minute); !ok {
		t.Fatal("first attempt refused")
	}
	if ok, _ := c.ReserveAttempt(ctx, key, 1, time.Minute); ok {
		t.Fatal("second attempt admitted over a limit of one")
	}
	if err := c.ReleaseAttempt(ctx, key); err != nil {
		t.Fatalf("release: %v", err)
	}
	if err := c.ReleaseAttempt(ctx, key); err != nil {
		t.Fatalf("release: %v", err)
	}
	if n := c.Exists(ctx, key).Val(); n != 0 {
		t.Fatal("a counter released to zero was kept")
	}
	// Releasing a key that is already gone leaves nothing behind.
	if err := c.ReleaseAttempt(ctx, key); err != nil {
		t.Fatalf("release: %v", err)
	}
	if n := c.Exists(ctx, key).Val(); n != 0 {
		t.Fatal("releasing a missing key created one")
	}
}
