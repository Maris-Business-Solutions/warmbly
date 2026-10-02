package unibox

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/models"
)

// gatedCompute counts computations and holds each one until release is closed.
type gatedCompute struct {
	calls     atomic.Int32
	release   chan struct{}
	started   chan overviewScope
	cancelled chan struct{}
}

func newGatedCompute() *gatedCompute {
	return &gatedCompute{release: make(chan struct{}), started: make(chan overviewScope, 64), cancelled: make(chan struct{}, 64)}
}

func (g *gatedCompute) fn(ctx context.Context, scope overviewScope) (*models.UniboxOverview, error) {
	g.calls.Add(1)
	g.started <- scope
	select {
	case <-g.release:
	case <-ctx.Done():
		g.cancelled <- struct{}{}
		return nil, ctx.Err()
	}
	// Total carries the org so a caller can tell whose counts it got.
	return &models.UniboxOverview{Total: int64(scope.OrgID.ID())}, nil
}

// waitForWaiters blocks until scope's flight has n callers waiting on it.
func waitForWaiters(c *overviewCache, scope overviewScope, n int) {
	for {
		c.mu.Lock()
		f := c.flights[scope]
		got := f != nil && f.waiters == n
		c.mu.Unlock()
		if got {
			return
		}
		time.Sleep(time.Millisecond)
	}
}

func TestOverviewCacheSharesOneComputationPerScope(t *testing.T) {
	g := newGatedCompute()
	c := newOverviewCache(g.fn)
	org := uuid.New()

	var wg sync.WaitGroup
	results := make([]*models.UniboxOverview, 8)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			o, err := c.get(context.Background(), overviewScope{OrgID: org})
			if err != nil {
				t.Errorf("get: %v", err)
			}
			results[i] = o
		}(i)
	}
	<-g.started
	// Let every caller reach the flight before it lands.
	time.Sleep(20 * time.Millisecond)
	close(g.release)
	wg.Wait()

	if n := g.calls.Load(); n != 1 {
		t.Fatalf("eight concurrent callers ran %d computations, want 1", n)
	}
	for _, o := range results {
		if o != results[0] {
			t.Fatal("callers of one flight got different results")
		}
	}
	if _, err := c.get(context.Background(), overviewScope{OrgID: org}); err != nil || g.calls.Load() != 1 {
		t.Fatalf("a read inside the freshness window recomputed (calls=%d, err=%v)", g.calls.Load(), err)
	}
}

func TestOverviewCacheNeverSharesAcrossOrganizations(t *testing.T) {
	g := newGatedCompute()
	close(g.release)
	c := newOverviewCache(g.fn)
	a, b := uuid.New(), uuid.New()

	oa, err := c.get(context.Background(), overviewScope{OrgID: a})
	if err != nil {
		t.Fatal(err)
	}
	ob, err := c.get(context.Background(), overviewScope{OrgID: b})
	if err != nil {
		t.Fatal(err)
	}
	if g.calls.Load() != 2 {
		t.Fatalf("two organizations ran %d computations, want 2", g.calls.Load())
	}
	if oa.Total != int64(a.ID()) || ob.Total != int64(b.ID()) {
		t.Fatal("an organization was served another organization's overview")
	}
}

func TestOverviewCacheCallerLeavesWithoutCancellingTheShare(t *testing.T) {
	g := newGatedCompute()
	c := newOverviewCache(g.fn)
	scope := overviewScope{OrgID: uuid.New()}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := c.get(ctx, scope)
		done <- err
	}()
	<-g.started
	waiter := make(chan *models.UniboxOverview, 1)
	go func() {
		o, _ := c.get(context.Background(), scope)
		waiter <- o
	}()
	waitForWaiters(c, scope, 2)
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("the departing caller got %v, want context.Canceled", err)
	}
	close(g.release)
	if o := <-waiter; o == nil {
		t.Fatal("the remaining caller lost its result when the first one left")
	}
	if g.calls.Load() != 1 {
		t.Fatalf("ran %d computations, want 1", g.calls.Load())
	}
}

func TestOverviewCacheCancelsAFlightNobodyWaitsFor(t *testing.T) {
	g := newGatedCompute()
	c := newOverviewCache(g.fn)
	scope := overviewScope{OrgID: uuid.New()}

	ctxA, cancelA := context.WithCancel(context.Background())
	ctxB, cancelB := context.WithCancel(context.Background())
	errs := make(chan error, 2)
	go func() { _, err := c.get(ctxA, scope); errs <- err }()
	<-g.started
	go func() { _, err := c.get(ctxB, scope); errs <- err }()
	waitForWaiters(c, scope, 2)

	cancelA()
	<-errs
	select {
	case <-g.cancelled:
		t.Fatal("the flight was cancelled while a caller still waited on it")
	case <-time.After(30 * time.Millisecond):
	}

	cancelB()
	<-errs
	select {
	case <-g.cancelled:
	case <-time.After(time.Second):
		t.Fatal("the flight kept running after every caller left")
	}

	close(g.release)
	if _, err := c.get(context.Background(), scope); err != nil {
		t.Fatal(err)
	}
	if n := g.calls.Load(); n != 2 {
		t.Fatalf("an abandoned flight's result was reused (calls=%d, want 2)", n)
	}
}

func TestOverviewCacheRecomputesOnceStaleOrForgotten(t *testing.T) {
	g := newGatedCompute()
	close(g.release)
	c := newOverviewCache(g.fn)
	now := time.Now()
	c.now = func() time.Time { return now }
	org := uuid.New()
	scope := overviewScope{OrgID: org}

	_, _ = c.get(context.Background(), scope)
	_, _ = c.get(context.Background(), scope)
	if g.calls.Load() != 1 {
		t.Fatalf("calls=%d, want 1", g.calls.Load())
	}

	now = now.Add(overviewFreshFor)
	_, _ = c.get(context.Background(), scope)
	if g.calls.Load() != 2 {
		t.Fatalf("a stale entry was served (calls=%d)", g.calls.Load())
	}

	c.forget(org)
	_, _ = c.get(context.Background(), scope)
	if g.calls.Load() != 3 {
		t.Fatalf("a forgotten entry was served (calls=%d)", g.calls.Load())
	}

	now = now.Add(2 * overviewFreshFor)
	_, _ = c.get(context.Background(), overviewScope{OrgID: uuid.New()})
	c.mu.Lock()
	_, kept := c.entries[scope]
	c.mu.Unlock()
	if kept {
		t.Fatal("an expired entry survived the sweep")
	}
}

func TestOverviewCacheTurnsAPanicIntoAnError(t *testing.T) {
	c := newOverviewCache(func(context.Context, overviewScope) (*models.UniboxOverview, error) {
		panic("boom")
	})
	if _, err := c.get(context.Background(), overviewScope{OrgID: uuid.New()}); err == nil {
		t.Fatal("a panicking computation returned no error")
	}
}
