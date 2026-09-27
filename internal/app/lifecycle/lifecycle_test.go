package lifecycle

import (
	"testing"
	"time"

	"github.com/warmbly/warmbly/internal/models"
)

func TestDecideRestsOnRealTrouble(t *testing.T) {
	now := time.Now()
	for _, h := range []models.WarmupHealthState{
		models.WarmupHealthQuarantined, models.WarmupHealthBlocked,
	} {
		d := Decide(models.SendLifecycleActive, nil, h, now)
		if d.Next != models.SendLifecycleResting {
			t.Errorf("health %q gave %q, want resting", h, d.Next)
		}
		if d.Reason == "" {
			t.Errorf("health %q rested with no reason", h)
		}
	}
}

// Watch and throttled are spam placement, which only slows a mailbox down: its
// cold budget is dampened, and it stays in rotation.
func TestDecideDoesNotRestOnPlacementBands(t *testing.T) {
	for _, h := range []models.WarmupHealthState{models.WarmupHealthWatch, models.WarmupHealthThrottled} {
		if d := Decide(models.SendLifecycleActive, nil, h, time.Now()); d.Next != models.SendLifecycleActive {
			t.Errorf("%s gave %q, want active", h, d.Next)
		}
	}
}

func TestDecideResumesOnlyAfterProbation(t *testing.T) {
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)

	served := now.Add(-models.RestProbation)
	if d := Decide(models.SendLifecycleResting, &served, models.WarmupHealthHealthy, now); d.Next != models.SendLifecycleActive {
		t.Errorf("a recovered mailbox that served its rest gave %q, want active", d.Next)
	}

	fresh := now.Add(-time.Hour)
	if d := Decide(models.SendLifecycleResting, &fresh, models.WarmupHealthHealthy, now); d.Next != models.SendLifecycleResting {
		t.Errorf("an hour of rest gave %q, want it still resting", d.Next)
	}

	if d := Decide(models.SendLifecycleResting, &served, models.WarmupHealthQuarantined, now); d.Next != models.SendLifecycleResting {
		t.Errorf("a still-quarantined mailbox gave %q, want resting", d.Next)
	}
	if d := Decide(models.SendLifecycleResting, &served, models.WarmupHealthThrottled, now); d.Next != models.SendLifecycleActive {
		t.Errorf("a throttled mailbox that served its rest gave %q, want active at its dampened volume", d.Next)
	}
}

// Reserve is the owner's decision, in both directions.
func TestDecideNeverTouchesReserve(t *testing.T) {
	now := time.Now()
	for _, h := range []models.WarmupHealthState{
		models.WarmupHealthHealthy, models.WarmupHealthThrottled, models.WarmupHealthBlocked,
	} {
		if d := Decide(models.SendLifecycleReserve, nil, h, now); d.Next != models.SendLifecycleReserve {
			t.Errorf("health %q moved a reserved mailbox to %q", h, d.Next)
		}
	}
}

// A row written before the column existed reads as empty and must be treated
// as active rather than left in limbo.
func TestDecideTreatsAnUnsetStateAsActive(t *testing.T) {
	if d := Decide("", nil, models.WarmupHealthHealthy, time.Now()); d.Next != models.SendLifecycleActive {
		t.Errorf("an unset lifecycle gave %q, want active", d.Next)
	}
}

// The bug this guards: probation has to measure time back in the pool. A
// mailbox that sat resting and quarantined for a week would otherwise resume on
// its first tick out, having served no probation at all.
func TestDecideRestartsProbationWhileStillOut(t *testing.T) {
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	longAgo := now.Add(-10 * 24 * time.Hour)

	d := Decide(models.SendLifecycleResting, &longAgo, models.WarmupHealthQuarantined, now)
	if d.Next != models.SendLifecycleResting {
		t.Errorf("next = %q, want it still resting", d.Next)
	}
	if !d.RestartProbation {
		t.Error("a still-quarantined mailbox must restart its probation, not bank the time")
	}

	// Once healthy, the clock runs and is not restarted.
	healthy := Decide(models.SendLifecycleResting, &longAgo, models.WarmupHealthHealthy, now)
	if healthy.RestartProbation {
		t.Error("a healthy mailbox must not have its probation restarted")
	}
	if healthy.Next != models.SendLifecycleActive {
		t.Errorf("next = %q, want active after a served probation", healthy.Next)
	}
}

// A mailbox that is not resting has no probation to restart.
func TestDecideDoesNotRestartProbationForOtherStates(t *testing.T) {
	now := time.Now()
	for _, state := range []models.SendLifecycle{
		models.SendLifecycleActive, models.SendLifecycleReserve, "",
	} {
		if d := Decide(state, nil, models.WarmupHealthWatch, now); d.RestartProbation {
			t.Errorf("state %q asked to restart probation", state)
		}
	}
}

// A mailbox that is in no warmup pool reports no health at all. Reading that
// as healthy would let a rested mailbox resume on the strength of having left
// the pool, so it serves the rest window first. It must not restart the
// window either: with no warmup running it is not recovering, and holding it
// until a signal that never comes strands it (issue #243).
func TestUnknownHealthResumesOnlyAfterTheRestWindow(t *testing.T) {
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)

	fresh := now.Add(-time.Hour)
	d := Decide(models.SendLifecycleResting, &fresh, "", now)
	if d.Next != models.SendLifecycleResting {
		t.Fatalf("resting mailbox with no pool resumed after an hour: %v", d.Next)
	}
	if d.RestartProbation {
		t.Fatal("no health signal must let the clock run, not restart it")
	}

	served := now.Add(-models.RestProbation)
	d = Decide(models.SendLifecycleResting, &served, "", now)
	if d.Next != models.SendLifecycleActive {
		t.Fatalf("resting mailbox with no pool stayed %v after the rest window", d.Next)
	}
	if d.Reason == "" {
		t.Fatal("resuming with no signal needs a reason the owner can read")
	}

	if d := Decide(models.SendLifecycleActive, &served, "", now); d.Next != models.SendLifecycleActive {
		t.Fatalf("active mailbox with no pool moved to %v", d.Next)
	}
}

// A resting mailbox with no clock cannot serve a window, so it stays put
// rather than resuming on a missing stamp.
func TestUnknownHealthWithNoClockStaysResting(t *testing.T) {
	if d := Decide(models.SendLifecycleResting, nil, "", time.Now()); d.Next != models.SendLifecycleResting {
		t.Fatalf("resting mailbox with no since resumed: %v", d.Next)
	}
}
