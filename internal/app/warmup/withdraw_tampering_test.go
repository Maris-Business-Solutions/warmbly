package warmup

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

// withdrawRepo holds one standing and the strikes left in its deciding window,
// and records how the hold was revised.
type withdrawRepo struct {
	ownPoolRepo
	hold      *repository.WarmupHold
	exists    bool
	removed   bool
	deletions int
	spamFlags int
	excluded  string
	byAddress bool

	revised      bool
	revisedState models.WarmupHealthState
	revisedUntil *time.Time
	window       [2]time.Time
}

func (r *withdrawRepo) HasWarmupTampering(context.Context, uuid.UUID, string, string) (bool, error) {
	return r.exists, nil
}

func (r *withdrawRepo) WithdrawWarmupTampering(context.Context, uuid.UUID, string, string) (bool, error) {
	r.removed = true
	return true, nil
}

func (r *withdrawRepo) GetWarmupHold(context.Context, uuid.UUID) (*repository.WarmupHold, error) {
	return r.hold, nil
}

func (r *withdrawRepo) CountWarmupTamperingBetween(_ context.Context, _ uuid.UUID, from, to time.Time, exclude string, byAddress bool) (int, int, error) {
	r.window = [2]time.Time{from, to}
	r.excluded, r.byAddress = exclude, byAddress
	return r.deletions, r.spamFlags, nil
}

func (r *withdrawRepo) ReviseWarmupHold(_ context.Context, _ uuid.UUID, hold *repository.WarmupHold, state models.WarmupHealthState, until *time.Time, _ string) (bool, error) {
	if hold != r.hold {
		return false, nil
	}
	r.revised, r.revisedState, r.revisedUntil = true, state, until
	return true, nil
}

func tamperingHold(state models.WarmupHealthState, decidedAgo time.Duration, reason string) *repository.WarmupHold {
	at := time.Now().Add(-decidedAgo)
	term := warmupQuarantineDuration
	if state == models.WarmupHealthBlocked {
		term = warmupBlockDuration
	}
	until := at.Add(term)
	return &repository.WarmupHold{State: state, BlockedAt: &at, BlockedUntil: &until, Reason: reason, InPool: true}
}

// A hold is re-decided on the strikes left in the seven days before it was
// imposed, not on today's window, so strikes that aged out still count.
func TestWithdrawTamperingRevisesOnTheWindowThatDecidedTheHold(t *testing.T) {
	pause := tamperingPausePrefix + "2 warmup emails deleted in the last 7 days."
	block := tamperingBlockPrefix + "5 warmup emails deleted in the last 7 days."
	cases := []struct {
		name      string
		hold      *repository.WarmupHold
		deletions int
		want      models.WarmupHealthState // "" means left alone
		wantUntil bool
	}{
		{"a pause left with one strike lifts", tamperingHold(models.WarmupHealthQuarantined, 24*time.Hour, pause), 1, models.WarmupHealthHealthy, false},
		{"a pause left with two strikes stands", tamperingHold(models.WarmupHealthQuarantined, 24*time.Hour, pause), 2, "", false},
		{"an old block left with four strikes stands", tamperingHold(models.WarmupHealthBlocked, 10*24*time.Hour, block), 4, "", false},
		{"a block left with three strikes becomes a pause from when it was decided", tamperingHold(models.WarmupHealthBlocked, 2*24*time.Hour, block), 3, models.WarmupHealthQuarantined, true},
		{"a block whose pause would already have ended lifts", tamperingHold(models.WarmupHealthBlocked, 10*24*time.Hour, block), 3, models.WarmupHealthHealthy, false},
		{"a complaint quarantine is not ours to lift", tamperingHold(models.WarmupHealthQuarantined, 24*time.Hour, "complaint rate 0.20% exceeded quarantine threshold"), 0, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &withdrawRepo{
				ownPoolRepo: ownPoolRepo{health: &models.WarmupParticipantHealth{PoolType: "premium", HealthState: tc.hold.State}},
				hold:        tc.hold, exists: true, deletions: tc.deletions,
			}
			if _, err := NewService(repo).WithdrawTampering(context.Background(), uuid.New(), "<m@example.test>", "deletion"); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !repo.window[1].IsZero() && (repo.excluded != "<m@example.test>" || repo.byAddress) {
				t.Fatalf("counted excluding %q by address %v; want the withdrawn strike left out of this mailbox's count", repo.excluded, repo.byAddress)
			}
			if !repo.removed {
				t.Fatal("the strike was not withdrawn")
			}
			if tc.want == "" {
				if repo.revised {
					t.Fatalf("revised to %v, want the hold left alone", repo.revisedState)
				}
				return
			}
			if !repo.revised || repo.revisedState != tc.want {
				t.Fatalf("revised %v to %v, want %v", repo.revised, repo.revisedState, tc.want)
			}
			if (repo.revisedUntil != nil) != tc.wantUntil {
				t.Fatalf("revised until %v, want a term %v", repo.revisedUntil, tc.wantUntil)
			}
			if tc.wantUntil && !repo.revisedUntil.Equal(tc.hold.BlockedAt.Add(warmupQuarantineDuration)) {
				t.Fatalf("pause ends %v, want seven days from when the block was decided", repo.revisedUntil)
			}
			if !repo.window[1].Equal(*tc.hold.BlockedAt) || !repo.window[0].Equal(tc.hold.BlockedAt.Add(-7*24*time.Hour)) {
				t.Fatalf("counted strikes over %v, want the seven days before the hold", repo.window)
			}
		})
	}
}

// A mailbox out of every pool has its address's ledger standing revised, on
// the strikes of every mailbox sharing the address.
func TestWithdrawTamperingRevisesTheLedger(t *testing.T) {
	pause := tamperingPausePrefix + "2 warmup emails deleted in the last 7 days."
	hold := tamperingHold(models.WarmupHealthQuarantined, 24*time.Hour, pause)
	hold.InPool = false
	repo := &withdrawRepo{hold: hold, exists: true, deletions: 1}
	health, err := NewService(repo).WithdrawTampering(context.Background(), uuid.New(), "<m@example.test>", "deletion")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.revised || repo.revisedState != models.WarmupHealthHealthy || !repo.byAddress {
		t.Fatalf("revised %v to %v by address %v, want the ledger hold lifted on the address's strikes", repo.revised, repo.revisedState, repo.byAddress)
	}
	if health != nil {
		t.Fatalf("a mailbox in no pool came back with a standing: %+v", health)
	}
}

// Nothing to withdraw touches nothing.
func TestWithdrawTamperingWithoutAStrikeIsANoop(t *testing.T) {
	repo := &withdrawRepo{hold: tamperingHold(models.WarmupHealthQuarantined, time.Hour, tamperingPausePrefix+"x")}
	if _, err := NewService(repo).WithdrawTampering(context.Background(), uuid.New(), "<m@example.test>", "deletion"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.revised || repo.removed || repo.updated {
		t.Fatalf("revised %v, removed %v, evaluated %v; want nothing", repo.revised, repo.removed, repo.updated)
	}
}
