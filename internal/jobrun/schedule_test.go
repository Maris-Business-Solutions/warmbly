package jobrun

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/warmbly/warmbly/internal/models"
)

// memStore mirrors the repository's scheduling SQL in memory.
type memStore struct {
	mu  sync.Mutex
	due map[string]time.Time
}

func newMemStore() *memStore { return &memStore{due: map[string]time.Time{}} }

func (m *memStore) Register(_ context.Context, name, _ string, _ time.Duration, next, earliest time.Time) (time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	stored, ok := m.due[name]
	if !ok {
		m.due[name] = next
		return next, nil
	}
	if next.Before(stored) {
		stored = next
	}
	if stored.Before(earliest) {
		stored = earliest
	}
	m.due[name] = stored
	return stored, nil
}

func (m *memStore) Claim(_ context.Context, name string, due, next time.Time) (bool, time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	stored, ok := m.due[name]
	if !ok || stored.Equal(due) {
		m.due[name] = next
		return true, next, nil
	}
	return false, stored, nil
}

func (m *memStore) set(name string, due time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.due[name] = due.Truncate(time.Microsecond)
}

func (m *memStore) MarkStarted(context.Context, string, time.Time) error { return nil }
func (m *memStore) MarkFinished(context.Context, string, time.Time, time.Time, error) error {
	return nil
}
func (m *memStore) RequestRun(context.Context, string) (bool, error)     { return false, nil }
func (m *memStore) TakeRunRequest(context.Context, string) (bool, error) { return false, nil }
func (m *memStore) List(context.Context) ([]models.ScheduledJobRun, error) {
	return nil, nil
}

func useStore(t *testing.T, s Store) {
	t.Helper()
	prev, prevSvc := current()
	Configure(s, "test")
	t.Cleanup(func() { Configure(prev, prevSvc) })
}

// startLoop runs a Loop that counts its runs and returns a stop function that
// waits for it to exit.
func startLoop(name string, interval time.Duration, runOnBoot bool, runs *atomic.Int32) func() {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		Loop(ctx, name, interval, runOnBoot, func(context.Context) error {
			runs.Add(1)
			return nil
		})
	}()
	return func() {
		cancel()
		<-done
	}
}

// Restarting more often than the interval used to push the first run back by
// a full interval every time, so the job never ran at all.
func TestLoopIsNotStarvedByRestartsShorterThanItsInterval(t *testing.T) {
	store := newMemStore()
	useStore(t, store)

	const name = "a"
	interval := 600 * time.Millisecond
	offset := phaseOffset(name, interval)
	restartEvery := offset + (interval-offset)/2

	var runs atomic.Int32
	for i := 0; i < 8; i++ {
		stop := startLoop(name, interval, false, &runs)
		time.Sleep(restartEvery)
		stop()
	}
	if got := runs.Load(); got < 2 {
		t.Fatalf("ran %d times across 8 restarts every %s at a %s interval, want at least 2", got, restartEvery, interval)
	}
}

func TestLoopRunsAnOverdueJobRightAfterItsOffset(t *testing.T) {
	store := newMemStore()
	useStore(t, store)

	const name = "overdue"
	interval := 2 * time.Second
	store.set(name, time.Now().Add(-time.Hour))

	var runs atomic.Int32
	stop := startLoop(name, interval, false, &runs)
	defer stop()

	deadline := time.Now().Add(phaseOffset(name, interval) + 300*time.Millisecond)
	for runs.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if runs.Load() != 1 {
		t.Fatalf("overdue job ran %d times shortly after boot, want once", runs.Load())
	}
}

func TestLoopWaitsOnlyTheRemainderOfAnInterval(t *testing.T) {
	store := newMemStore()
	useStore(t, store)

	const name = "remaining"
	interval := 5 * time.Second
	offset := phaseOffset(name, interval)
	store.set(name, time.Now().Add(offset+200*time.Millisecond))

	var runs atomic.Int32
	stop := startLoop(name, interval, false, &runs)
	defer stop()

	time.Sleep(offset + 100*time.Millisecond)
	if runs.Load() != 0 {
		t.Fatalf("ran before its stored due time")
	}
	time.Sleep(400 * time.Millisecond)
	if runs.Load() != 1 {
		t.Fatalf("ran %d times by its stored due time, want once without a fresh interval", runs.Load())
	}
}

func TestLoopGivesANewJobItsFullFirstInterval(t *testing.T) {
	store := newMemStore()
	useStore(t, store)

	const name = "fresh"
	interval := 600 * time.Millisecond
	offset := phaseOffset(name, interval)

	var runs atomic.Int32
	stop := startLoop(name, interval, false, &runs)
	defer stop()

	time.Sleep(offset + interval - 100*time.Millisecond)
	if runs.Load() != 0 {
		t.Fatalf("new job ran before its first interval")
	}
	time.Sleep(250 * time.Millisecond)
	if runs.Load() != 1 {
		t.Fatalf("new job ran %d times after its first interval, want once", runs.Load())
	}
}

// Two processes hosting the same job share its slots instead of each running it.
func TestLoopRunsEachSlotOnceAcrossInstances(t *testing.T) {
	store := newMemStore()
	useStore(t, store)

	const name = "shared"
	interval := 200 * time.Millisecond
	store.set(name, time.Now().Add(-time.Second))

	var runs atomic.Int32
	stopA := startLoop(name, interval, false, &runs)
	stopB := startLoop(name, interval, false, &runs)
	time.Sleep(phaseOffset(name, interval) + 5*interval + interval/2)
	stopA()
	stopB()

	if got := runs.Load(); got < 4 || got > 7 {
		t.Fatalf("two instances ran %d times over about six slots, want one run per slot", got)
	}
}

func TestNextDueKeepsTheCadenceAndDropsMissedSlots(t *testing.T) {
	due := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		at   time.Time
		want time.Time
	}{
		{due, due.Add(time.Hour)},
		{due.Add(10 * time.Minute), due.Add(time.Hour)},
		{due.Add(time.Hour), due.Add(2 * time.Hour)},
		{due.Add(150 * time.Minute), due.Add(3 * time.Hour)},
	}
	for _, c := range cases {
		if got := nextDue(due, time.Hour, c.at); !got.Equal(c.want) {
			t.Errorf("nextDue at %s = %s, want %s", c.at, got, c.want)
		}
	}
}
