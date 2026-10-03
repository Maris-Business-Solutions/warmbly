package campaign

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestReadCacheGetOrComputeCoalesces proves that many concurrent callers hitting
// a cold cache for the same key share a single computation instead of each
// starting their own planner walk.
func TestReadCacheGetOrComputeCoalesces(t *testing.T) {
	c := newReadCache[int](time.Minute)

	var calls int32
	release := make(chan struct{})
	start := make(chan struct{})

	const callers = 32
	var wg sync.WaitGroup
	results := make([]int, callers)
	errs := make([]error, callers)

	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			v, err := c.getOrCompute("k", func() (int, error) {
				atomic.AddInt32(&calls, 1)
				// Hold the flight open so every caller is in flight at once.
				<-release
				return 42, nil
			})
			results[idx] = v
			errs[idx] = err
		}(i)
	}

	close(start)
	// Give the goroutines time to pile onto the one in-flight computation.
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected exactly one computation, got %d", got)
	}
	for i := 0; i < callers; i++ {
		if errs[i] != nil {
			t.Fatalf("caller %d returned error: %v", i, errs[i])
		}
		if results[i] != 42 {
			t.Fatalf("caller %d got %d, want 42", i, results[i])
		}
	}
}

// TestReadCacheGetOrComputeCachesResult proves a successful result is cached so
// a later caller is served without recomputing, and that a different key is
// computed independently.
func TestReadCacheGetOrComputeCachesResult(t *testing.T) {
	c := newReadCache[int](time.Minute)

	var calls int32
	compute := func(v int) func() (int, error) {
		return func() (int, error) {
			atomic.AddInt32(&calls, 1)
			return v, nil
		}
	}

	if v, err := c.getOrCompute("a", compute(1)); err != nil || v != 1 {
		t.Fatalf("first call: v=%d err=%v", v, err)
	}
	if v, err := c.getOrCompute("a", compute(99)); err != nil || v != 1 {
		t.Fatalf("second call should be cached: v=%d err=%v", v, err)
	}
	if v, err := c.getOrCompute("b", compute(2)); err != nil || v != 2 {
		t.Fatalf("different key: v=%d err=%v", v, err)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected 2 computations (one per key), got %d", got)
	}
}

// TestReadCacheGetOrComputeDoesNotCacheErrors proves an error is not cached, so
// the next caller retries rather than being served a failure.
func TestReadCacheGetOrComputeDoesNotCacheErrors(t *testing.T) {
	c := newReadCache[int](time.Minute)
	sentinel := errors.New("boom")

	if _, err := c.getOrCompute("k", func() (int, error) { return 0, sentinel }); !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
	v, err := c.getOrCompute("k", func() (int, error) { return 7, nil })
	if err != nil || v != 7 {
		t.Fatalf("retry after error should recompute: v=%d err=%v", v, err)
	}
}

// TestReadCacheGetOrComputeRecoversPanic proves a panic in compute is turned
// into an error and releases waiters rather than deadlocking the flight.
func TestReadCacheGetOrComputeRecoversPanic(t *testing.T) {
	c := newReadCache[int](time.Minute)

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, err := c.getOrCompute("k", func() (int, error) { panic("kaboom") })
		if err == nil {
			t.Errorf("expected an error from a panicking compute")
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("getOrCompute deadlocked after compute panicked")
	}
}
