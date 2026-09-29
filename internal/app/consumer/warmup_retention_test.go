package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	warmupapp "github.com/warmbly/warmbly/internal/app/warmup"
	"github.com/warmbly/warmbly/internal/config"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

// retentionWarmupRepo answers the one receipt lookup the removal and flag
// handlers make.
type retentionWarmupRepo struct {
	repository.WarmupRepository
	rec *repository.WarmupReceived
}

func (r retentionWarmupRepo) GetWarmupReceived(context.Context, uuid.UUID, uuid.UUID) (*repository.WarmupReceived, error) {
	return r.rec, nil
}

// retentionWarmupService records which strikes the handlers asked for.
type retentionWarmupService struct {
	warmupapp.Service
	strikes []string
}

func (s *retentionWarmupService) RecordTampering(_ context.Context, _ uuid.UUID, _, kind string) (*models.WarmupParticipantHealth, *errx.Error) {
	s.strikes = append(s.strikes, kind)
	return nil, nil
}

func (s *retentionWarmupService) WithdrawTampering(_ context.Context, _ uuid.UUID, _, kind string) (*models.WarmupParticipantHealth, *errx.Error) {
	s.strikes = append(s.strikes, "withdraw:"+kind)
	return nil, nil
}

func (s *retentionWarmupService) ApplySpamReport(context.Context, uuid.UUID, uuid.UUID, string, string) (*models.WarmupParticipantHealth, *errx.Error) {
	s.strikes = append(s.strikes, "spam_report")
	return nil, nil
}

func retentionService(rec *repository.WarmupReceived) (*JobsService, *retentionWarmupService) {
	svc := &retentionWarmupService{}
	return &JobsService{
		WarmupRepo:      retentionWarmupRepo{rec: rec},
		WarmupService:   svc,
		EmailRepository: warmupInboxEmailRepo{},
	}, svc
}

func receivedAgo(age time.Duration) *repository.WarmupReceived {
	return &repository.WarmupReceived{
		EmailAccountID: uuid.New(), InternalID: uuid.New(),
		MessageID: "<warmup@example.test>", SenderAccountID: uuid.New(),
		CreatedAt: time.Now().Add(-age),
	}
}

func TestWarmupDeletionCounts(t *testing.T) {
	now := time.Now()
	window := time.Duration(config.WarmupDeletionStrikeHours) * time.Hour
	retired := now.Add(-time.Hour)
	cases := []struct {
		name string
		rec  *repository.WarmupReceived
		want bool
	}{
		{"no receipt is not warmup", nil, false},
		{"fresh is a strike", &repository.WarmupReceived{CreatedAt: now.Add(-time.Hour)}, true},
		{"just inside the window is a strike", &repository.WarmupReceived{CreatedAt: now.Add(-window + time.Minute)}, true},
		{"past the window is housekeeping", &repository.WarmupReceived{CreatedAt: now.Add(-window - time.Minute)}, false},
		{"weeks later is housekeeping", &repository.WarmupReceived{CreatedAt: now.AddDate(0, 0, -31)}, false},
		{"retired by the platform is never a strike, even fresh", &repository.WarmupReceived{CreatedAt: now.Add(-time.Hour), RetiredAt: &retired}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := warmupDeletionCounts(tc.rec, now); got != tc.want {
				t.Fatalf("warmupDeletionCounts() = %v, want %v", got, tc.want)
			}
		})
	}
}

// A removal of warmup mail is judged only while the message is fresh, and then
// never on the removal itself: the worker is asked where the message went.
// Later it is the mailbox owner tidying, Gmail purging its Trash, a server
// retention rule, or the platform's own retention, none of which is harm.
func TestRemoveEmailChecksOnlyFreshWarmupMail(t *testing.T) {
	cases := []struct {
		name   string
		rec    *repository.WarmupReceived
		checks int
	}{
		{"removed an hour after arrival", receivedAgo(time.Hour), 1},
		{"removed a week after arrival", receivedAgo(7 * 24 * time.Hour), 0},
		{"not warmup at all", nil, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, svc := retentionService(tc.rec)
			pub := &backfillPublisher{}
			worker := uuid.New()
			s.Publisher = pub
			s.EmailRepository = &backfillEmailRepo{account: &models.Email{WorkerID: &worker}}
			if err := s.HandleRemoveEmail(context.Background(), &models.JobEventRemoveEmail{
				UserID: uuid.New(), EmailID: uuid.New(), ID: uuid.New(),
			}); err != nil {
				t.Fatal(err)
			}
			if len(svc.strikes) != 0 {
				t.Fatalf("a removal was charged before the mailbox was searched: %v", svc.strikes)
			}
			if len(pub.actions) != tc.checks {
				t.Fatalf("published %d checks, want %d", len(pub.actions), tc.checks)
			}
			if tc.checks == 1 {
				a := pub.actions[0]
				if len(a.Actions) != 1 || a.Actions[0] != models.WarmupActionVerifyRemoval ||
					a.RFCMessageID != tc.rec.MessageID || a.InternalID != tc.rec.InternalID.String() || pub.workers[0] != worker {
					t.Fatalf("check carried %+v to %v", a, pub.workers[0])
				}
			}
		})
	}
}

// A mailbox with no worker to search it is not charged.
func TestRemoveEmailWithNowhereToCheckChargesNothing(t *testing.T) {
	s, svc := retentionService(receivedAgo(time.Hour))
	pub := &backfillPublisher{}
	s.Publisher = pub
	if err := s.HandleRemoveEmail(context.Background(), &models.JobEventRemoveEmail{
		UserID: uuid.New(), EmailID: uuid.New(), ID: uuid.New(),
	}); err != nil {
		t.Fatal(err)
	}
	if len(svc.strikes) != 0 || len(pub.actions) != 0 {
		t.Fatalf("strikes %v, checks %d; want neither", svc.strikes, len(pub.actions))
	}
}

// The strike follows where the worker found the message: anywhere outside the
// trash withdraws it, the trash or nowhere records it, and a message the
// retention sweep has retired is the platform's deletion whatever the search
// says.
func TestRemovalCheckedJudgesOnWhereTheMessageIs(t *testing.T) {
	retired := time.Now().Add(-time.Minute)
	cases := []struct {
		name    string
		outcome string
		retired bool
		want    []string
	}{
		{"moved to another folder", models.WarmupRemovalPresent, false, []string{"withdraw:deletion"}},
		{"in the trash", models.WarmupRemovalTrashed, false, []string{"deletion"}},
		{"gone for good", models.WarmupRemovalGone, false, []string{"deletion"}},
		{"gone after the platform retired it", models.WarmupRemovalGone, true, []string{"withdraw:deletion"}},
		{"an answer this consumer does not know", "sideways", false, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := receivedAgo(time.Hour)
			if tc.retired {
				rec.RetiredAt = &retired
			}
			s, svc := retentionService(rec)
			if err := s.HandleWarmupRemovalChecked(context.Background(), &models.JobEventWarmupRemovalChecked{
				UserID: uuid.New(), EmailID: uuid.New(), InternalID: rec.InternalID.String(),
				RFCMessageID: rec.MessageID, Outcome: tc.outcome,
			}); err != nil {
				t.Fatal(err)
			}
			if len(svc.strikes) != len(tc.want) {
				t.Fatalf("strikes = %v, want %v", svc.strikes, tc.want)
			}
			for i := range tc.want {
				if svc.strikes[i] != tc.want[i] {
					t.Fatalf("strikes = %v, want %v", svc.strikes, tc.want)
				}
			}
		})
	}
}

// unverifiedRepo serves one listing of strikes recorded before removals were
// checked and records which were stamped.
type unverifiedRepo struct {
	repository.WarmupRepository
	rows     []repository.WarmupTamperingToVerify
	verified []string
}

func (r *unverifiedRepo) ListUnverifiedDeletions(context.Context, time.Time, int) ([]repository.WarmupTamperingToVerify, error) {
	return r.rows, nil
}

func (r *unverifiedRepo) MarkTamperingVerified(_ context.Context, _ uuid.UUID, messageID, _ string) error {
	r.verified = append(r.verified, messageID)
	return nil
}

// Every old strike is searched for once, and stamped only once its search is
// on the bus, so a publish that fails is offered again.
func TestRecheckTamperingSearchesEachOldStrikeOnce(t *testing.T) {
	internal := uuid.New()
	rows := []repository.WarmupTamperingToVerify{
		{EmailAccountID: uuid.New(), UserID: uuid.New(), WorkerID: uuid.New(), MessageID: "<a@example.test>", InternalID: &internal},
		{EmailAccountID: uuid.New(), UserID: uuid.New(), WorkerID: uuid.New(), MessageID: "<b@example.test>"},
	}
	repo := &unverifiedRepo{rows: rows}
	pub := &backfillPublisher{}
	s := &JobsService{WarmupRepo: repo, Publisher: pub}
	if err := s.recheckTamperingBatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(pub.actions) != 2 || len(repo.verified) != 2 {
		t.Fatalf("published %d, stamped %d; want 2 and 2", len(pub.actions), len(repo.verified))
	}
	if pub.actions[0].InternalID != internal.String() || pub.actions[1].InternalID != "" || pub.workers[1] != rows[1].WorkerID {
		t.Fatalf("checks carried %+v to %v", pub.actions, pub.workers)
	}

	failing := &unverifiedRepo{rows: rows}
	s = &JobsService{WarmupRepo: failing, Publisher: &backfillPublisher{err: errors.New("bus down")}}
	if err := s.recheckTamperingBatch(context.Background()); err == nil {
		t.Fatal("a failed publish should be reported")
	}
	if len(failing.verified) != 0 {
		t.Fatalf("stamped %v without a search on the bus", failing.verified)
	}
}

func TestRemoveEmailNeverStrikesARetiredMessage(t *testing.T) {
	rec := receivedAgo(time.Hour)
	retired := time.Now().Add(-time.Minute)
	rec.RetiredAt = &retired
	s, svc := retentionService(rec)
	if err := s.HandleRemoveEmail(context.Background(), &models.JobEventRemoveEmail{
		UserID: uuid.New(), EmailID: uuid.New(), ID: uuid.New(),
	}); err != nil {
		t.Fatal(err)
	}
	if len(svc.strikes) != 0 {
		t.Fatalf("the platform's own deletion was recorded as tampering: %v", svc.strikes)
	}
}

// A fresh warmup message that the sync found in a folder the owner excluded
// from sync was filed, not deleted: it is still in the mailbox, so it is not
// a strike whatever its age.
func TestRemoveEmailNeverStrikesAMessageFiledIntoASkippedFolder(t *testing.T) {
	s, svc := retentionService(receivedAgo(time.Hour))
	if err := s.HandleRemoveEmail(context.Background(), &models.JobEventRemoveEmail{
		UserID: uuid.New(), EmailID: uuid.New(), ID: uuid.New(), SkippedFolder: "Warmer",
	}); err != nil {
		t.Fatal(err)
	}
	if len(svc.strikes) != 0 {
		t.Fatalf("a move into a skipped folder was recorded as tampering: %v", svc.strikes)
	}
}

// Gmail reports Delete as gaining the TRASH label. That is the owner's act
// and is judged on the same freshness rule; a spam flag is still the graver
// strike and is never subject to the window.
func TestFlagsAddJudgesGmailTrashOnFreshness(t *testing.T) {
	cases := []struct {
		name  string
		rec   *repository.WarmupReceived
		flags []string
		want  []string
	}{
		{"trashed an hour after arrival", receivedAgo(time.Hour), []string{"TRASH"}, []string{"deletion"}},
		{"trashed a month after arrival", receivedAgo(30 * 24 * time.Hour), []string{"TRASH"}, nil},
		{"flagged as spam a month after arrival", receivedAgo(30 * 24 * time.Hour), []string{"SPAM"}, []string{"spam_report", "spam_flag"}},
		{"read is not a strike", receivedAgo(time.Hour), []string{models.FlagSeen}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, svc := retentionService(tc.rec)
			if err := s.HandleFlagsAdd(context.Background(), &models.JobEventFlags{
				UserID: uuid.New(), EmailID: uuid.New(), ID: uuid.New(), Flags: tc.flags,
			}); err != nil {
				t.Fatal(err)
			}
			if len(svc.strikes) != len(tc.want) {
				t.Fatalf("strikes = %v, want %v", svc.strikes, tc.want)
			}
			for i := range tc.want {
				if svc.strikes[i] != tc.want[i] {
					t.Fatalf("strikes = %v, want %v", svc.strikes, tc.want)
				}
			}
		})
	}
}

// retentionPublisher captures the delete actions the sweep publishes.
type retentionPublisher struct {
	backfillPublisher
}

// retentionRepo serves one listing of each kind and records what was retired.
type retentionRepo struct {
	repository.WarmupRepository
	received []repository.WarmupMailToRetire
	sent     []repository.WarmupMailToRetire
	retired  []uuid.UUID
	pruned   *time.Time
}

func (r *retentionRepo) ListWarmupMailToRetire(context.Context, int, int) ([]repository.WarmupMailToRetire, error) {
	return r.received, nil
}

func (r *retentionRepo) ListWarmupSentCopiesToRetire(context.Context, int, int) ([]repository.WarmupMailToRetire, error) {
	return r.sent, nil
}

func (r *retentionRepo) RetireWarmupReceived(_ context.Context, _, internalID uuid.UUID) error {
	r.retired = append(r.retired, internalID)
	return nil
}

func (r *retentionRepo) RetireWarmupSentCopy(_ context.Context, token uuid.UUID) error {
	r.retired = append(r.retired, token)
	return nil
}

func (r *retentionRepo) PruneWarmupEventsBefore(_ context.Context, before time.Time) (int64, error) {
	r.pruned = &before
	return 0, nil
}

// The sweep publishes one delete per row, with every key the worker can find
// the message by, and retires the row only once the action is on the bus.
func TestRetireWarmupMailBatchPublishesAndRetires(t *testing.T) {
	worker := uuid.New()
	received := repository.WarmupMailToRetire{
		UserID: uuid.New(), EmailAccountID: uuid.New(), WorkerID: worker,
		InternalID: uuid.New(), MessageID: "<in@example.test>", ProviderKey: "gmail-1",
		Placement: models.WarmupPlacementFolder, Folder: "Reputation",
	}
	sent := repository.WarmupMailToRetire{
		UserID: uuid.New(), EmailAccountID: uuid.New(), WorkerID: worker,
		Token: uuid.New(), MessageID: "<out@example.test>",
	}
	repo := &retentionRepo{received: []repository.WarmupMailToRetire{received}, sent: []repository.WarmupMailToRetire{sent}}
	pub := &retentionPublisher{}
	s := &JobsService{WarmupRepo: repo, Publisher: pub}

	done, err := s.retireWarmupMailBatch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !done {
		t.Fatal("a short listing should complete the pass")
	}
	if len(pub.actions) != 2 {
		t.Fatalf("published %d actions, want 2", len(pub.actions))
	}
	in := pub.actions[0]
	if in.Actions[0] != models.WarmupActionDelete || in.GmailID != "gmail-1" || in.RFCMessageID != received.MessageID ||
		in.InternalID != received.InternalID.String() || in.TargetFolder != "Reputation" {
		t.Fatalf("received delete carried %+v", in)
	}
	out := pub.actions[1]
	if out.InternalID != "" || out.RFCMessageID != sent.MessageID || out.TargetFolder != config.WarmupFolderDefault {
		t.Fatalf("sent-copy delete carried %+v", out)
	}
	if len(repo.retired) != 2 || repo.retired[0] != received.InternalID || repo.retired[1] != sent.Token {
		t.Fatalf("retired %v, want the receipt then the token", repo.retired)
	}
}

func TestPruneWarmupEventsUsesTheInstanceWindow(t *testing.T) {
	repo := &retentionRepo{}
	s := &JobsService{WarmupRepo: repo, Publisher: &retentionPublisher{}}
	if err := s.pruneWarmupEvents(context.Background()); err != nil {
		t.Fatal(err)
	}
	if repo.pruned == nil {
		t.Fatal("nothing was pruned")
	}
	want := time.Now().AddDate(0, 0, -config.WarmupEventRetentionDaysDefault)
	if d := repo.pruned.Sub(want); d < -time.Minute || d > time.Minute {
		t.Fatalf("pruned before %v, want about %v", repo.pruned, want)
	}
}
