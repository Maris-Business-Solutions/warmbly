package unibox

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/models"
)

const (
	// overviewFreshFor stays below the dashboard's REFRESH_DELAY_MS so a post-event refresh never reads pre-event counts.
	overviewFreshFor = time.Second
	// overviewComputeTimeout bounds the shared computation, which no single caller's disconnect cancels.
	overviewComputeTimeout = 60 * time.Second
)

// overviewScope is every input the overview reads and so the cache key; a new narrowing input belongs here.
type overviewScope struct {
	OrgID uuid.UUID
}

type overviewFlight struct {
	started time.Time
	done    chan struct{}
	val     *models.UniboxOverview
	err     error
}

type overviewEntry struct {
	started time.Time
	val     *models.UniboxOverview
}

// overviewCache shares one computation per scope and serves it until overviewFreshFor after it started; values are read-only.
type overviewCache struct {
	compute func(context.Context, overviewScope) (*models.UniboxOverview, error)
	now     func() time.Time

	mu        sync.Mutex
	entries   map[overviewScope]overviewEntry
	flights   map[overviewScope]*overviewFlight
	lastSweep time.Time
}

func newOverviewCache(compute func(context.Context, overviewScope) (*models.UniboxOverview, error)) *overviewCache {
	return &overviewCache{
		compute: compute,
		now:     time.Now,
		entries: map[overviewScope]overviewEntry{},
		flights: map[overviewScope]*overviewFlight{},
	}
}

// get joins or starts a fresh computation; it returns on ctx's end while the computation carries on for others.
func (c *overviewCache) get(ctx context.Context, scope overviewScope) (*models.UniboxOverview, error) {
	c.mu.Lock()
	now := c.now()
	if e, ok := c.entries[scope]; ok && now.Sub(e.started) < overviewFreshFor {
		c.mu.Unlock()
		return e.val, nil
	}
	f, ok := c.flights[scope]
	if !ok || now.Sub(f.started) >= overviewFreshFor {
		f = &overviewFlight{started: now, done: make(chan struct{})}
		c.flights[scope] = f
		go c.run(context.WithoutCancel(ctx), scope, f)
	}
	c.mu.Unlock()

	select {
	case <-f.done:
		return f.val, f.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *overviewCache) run(detached context.Context, scope overviewScope, f *overviewFlight) {
	ctx, cancel := context.WithTimeout(detached, overviewComputeTimeout)
	defer cancel()
	val, err := c.safeCompute(ctx, scope)

	c.mu.Lock()
	f.val, f.err = val, err
	// A flight that was forgotten or superseded must not overwrite a newer answer.
	if c.flights[scope] == f {
		delete(c.flights, scope)
		if err == nil {
			c.entries[scope] = overviewEntry{started: f.started, val: val}
		}
	}
	c.sweepLocked()
	c.mu.Unlock()
	close(f.done)
}

func (c *overviewCache) safeCompute(ctx context.Context, scope overviewScope) (val *models.UniboxOverview, err error) {
	defer func() {
		if r := recover(); r != nil {
			val, err = nil, fmt.Errorf("unibox overview: panic: %v", r)
		}
	}()
	return c.compute(ctx, scope)
}

// forget makes the organization's next read compute afresh; callers already waiting keep their flight.
func (c *overviewCache) forget(orgID uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for scope := range c.entries {
		if scope.OrgID == orgID {
			delete(c.entries, scope)
		}
	}
	for scope := range c.flights {
		if scope.OrgID == orgID {
			delete(c.flights, scope)
		}
	}
}

// sweepLocked evicts expired entries, at most once per freshness window.
func (c *overviewCache) sweepLocked() {
	now := c.now()
	if now.Sub(c.lastSweep) < overviewFreshFor {
		return
	}
	c.lastSweep = now
	for scope, e := range c.entries {
		if now.Sub(e.started) >= overviewFreshFor {
			delete(c.entries, scope)
		}
	}
}
