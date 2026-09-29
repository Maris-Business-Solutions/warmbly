package warmup

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/models"
)

// withdrawRepo holds one participant row and whatever strikes remain after a
// withdrawal, and records whether the hold was lifted.
type withdrawRepo struct {
	ownPoolRepo
	deletionsLeft int
	lifted        string
}

func (r *withdrawRepo) WithdrawWarmupTampering(context.Context, uuid.UUID, string, string) (bool, error) {
	return true, nil
}

func (r *withdrawRepo) HealthMetricCounts(context.Context, uuid.UUID, time.Time, time.Time) (models.WarmupHealthCounts, error) {
	return models.WarmupHealthCounts{DeletionsLast7d: r.deletionsLeft}, nil
}

func (r *withdrawRepo) LiftTamperingHold(_ context.Context, _ uuid.UUID, reason string) (bool, error) {
	r.lifted = reason
	return true, nil
}

func heldRow(reason string) *models.WarmupParticipantHealth {
	until := time.Now().Add(6 * 24 * time.Hour)
	return &models.WarmupParticipantHealth{
		PoolType: "premium", HealthState: models.WarmupHealthQuarantined,
		BlockedUntil: &until, BlockedReason: &reason,
	}
}

// A pause that rested on a withdrawn strike is lifted; one the remaining
// strikes still earn, or one something else imposed, stays.
func TestWithdrawTamperingLiftsOnlyTheHoldItImposed(t *testing.T) {
	pause := tamperingPausePrefix + "2 warmup emails deleted in the last 7 days."
	cases := []struct {
		name      string
		reason    string
		left      int
		wantLift  bool
		wantState models.WarmupHealthState
	}{
		{"one strike left only warns", pause, 1, true, models.WarmupHealthWatch},
		{"two strikes left still pause", pause, 2, false, models.WarmupHealthQuarantined},
		{"a complaint quarantine is not ours to lift", "complaint rate 0.20% exceeded quarantine threshold", 0, false, models.WarmupHealthHealthy},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &withdrawRepo{ownPoolRepo: ownPoolRepo{health: heldRow(tc.reason)}, deletionsLeft: tc.left}
			health, err := NewService(repo).WithdrawTampering(context.Background(), uuid.New(), "<m@example.test>", "deletion")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if (repo.lifted != "") != tc.wantLift {
				t.Fatalf("lifted = %q, want lift %v", repo.lifted, tc.wantLift)
			}
			if tc.wantLift && repo.lifted != tc.reason {
				t.Fatalf("lifted the hold with reason %q, want %q", repo.lifted, tc.reason)
			}
			if health == nil || health.HealthState != tc.wantState {
				t.Fatalf("decided %v, want %v", health, tc.wantState)
			}
		})
	}
}
