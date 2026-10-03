package unibox

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestOverviewCacheJoinsASlowFlightPastFreshness is the reporter's #799 reproduction, flipped to
// the fixed behavior: a computation slower than overviewFreshFor is a single flight, and callers
// arriving after the freshness window has elapsed join it instead of starting overlapping ones.
func TestOverviewCacheJoinsASlowFlightPastFreshness(t *testing.T) {
	g := newGatedCompute()
	c := newOverviewCache(g.fn)
	var clock atomic.Int64
	clock.Store(int64(100 * time.Second))
	c.now = func() time.Time { return time.Unix(0, clock.Load()) }
	scope := overviewScope{OrgID: uuid.New()}

	// The first caller starts the one slow computation.
	first := make(chan error, 1)
	go func() { _, err := c.get(context.Background(), scope); first <- err }()
	<-g.started
	waitForWaiters(c, scope, 1)

	// Several more callers arrive well past the freshness window; each must join the in-flight
	// computation rather than start a second overlapping one.
	clock.Add(int64(5 * overviewFreshFor))
	const extra = 4
	rest := make(chan error, extra)
	for range extra {
		go func() { _, err := c.get(context.Background(), scope); rest <- err }()
	}
	waitForWaiters(c, scope, 1+extra)

	close(g.release)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	for range extra {
		if err := <-rest; err != nil {
			t.Fatal(err)
		}
	}
	if n := g.calls.Load(); n != 1 {
		t.Fatalf("a slow flight past freshness ran %d computations, want 1", n)
	}
}

// TestOverviewCacheForgetSupersedesAnActiveFlight checks explicit invalidation while a slow flight
// is running: forget drops the flight, the next caller starts exactly one replacement, the
// superseded flight's result is never cached, and a later read is served from the replacement.
func TestOverviewCacheForgetSupersedesAnActiveFlight(t *testing.T) {
	g := newGatedCompute()
	c := newOverviewCache(g.fn)
	c.now = func() time.Time { return time.Unix(0, int64(100*time.Second)) }
	scope := overviewScope{OrgID: uuid.New()}

	// A slow computation is in flight with a waiter.
	first := make(chan error, 1)
	go func() { _, err := c.get(context.Background(), scope); first <- err }()
	<-g.started
	waitForWaiters(c, scope, 1)

	// Explicit invalidation supersedes the active flight; the next caller starts one replacement.
	c.forget(scope.OrgID)
	second := make(chan error, 1)
	go func() { _, err := c.get(context.Background(), scope); second <- err }()
	<-g.started
	waitForWaiters(c, scope, 1)

	close(g.release)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if err := <-second; err != nil {
		t.Fatal(err)
	}
	if n := g.calls.Load(); n != 2 {
		t.Fatalf("forget admitted %d computations, want exactly 2 (original plus one replacement)", n)
	}

	// Only the replacement cached a result; a read inside the freshness window serves it with no
	// third computation, proving the superseded flight did not overwrite the newer answer.
	if _, err := c.get(context.Background(), scope); err != nil {
		t.Fatal(err)
	}
	if n := g.calls.Load(); n != 2 {
		t.Fatalf("a read after supersession recomputed (calls=%d, want 2)", n)
	}
}
