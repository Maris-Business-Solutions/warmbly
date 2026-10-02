package inboxtag

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/repository"
)

func TestFollowUp(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	ago := func(d int) time.Time { return now.AddDate(0, 0, -d) }

	cases := []struct {
		name  string
		state ThreadState
		want  string
	}{
		{
			"they replied and we have not answered for days",
			ThreadState{LastOutboundAt: ago(6), LastInboundAt: ago(3), BestIntent: IntentWantsInfo, LastKind: KindHumanReply},
			LabelNeedsReply,
		},
		{
			"they replied yesterday, which is not yet a delay worth flagging",
			ThreadState{LastOutboundAt: ago(2), LastInboundAt: ago(1), BestIntent: IntentWantsInfo, LastKind: KindHumanReply},
			"",
		},
		{
			"an unclassified inbound message is not treated as a human reply",
			ThreadState{LastOutboundAt: ago(6), LastInboundAt: ago(3)},
			"",
		},
		{
			"we sent recently and are simply waiting, which wears nothing",
			ThreadState{LastOutboundAt: ago(1)},
			"",
		},
		{
			"we sent a week ago and never heard back",
			ThreadState{LastOutboundAt: ago(7)},
			LabelFollowUp,
		},
		{
			// The one this whole feature is for.
			"they were interested, then went quiet",
			ThreadState{LastOutboundAt: ago(12), LastInboundAt: ago(14), BestIntent: IntentAgreed},
			LabelGoneQuiet,
		},
		{
			"an interested thread still inside its longer fuse",
			ThreadState{LastOutboundAt: ago(6), LastInboundAt: ago(8), BestIntent: IntentAgreed},
			"",
		},
		{
			// Chasing somebody who declined is rude; chasing somebody who asked
			// to be removed is a compliance problem.
			"they said no",
			ThreadState{LastOutboundAt: ago(30), LastInboundAt: ago(31), BestIntent: IntentNotInterested},
			"",
		},
		{
			"they asked to be removed",
			ThreadState{LastOutboundAt: ago(30), LastInboundAt: ago(31), BestIntent: IntentOptOut},
			"",
		},
		{
			"they said not now",
			ThreadState{LastOutboundAt: ago(40), LastInboundAt: ago(41), BestIntent: IntentNotNow},
			"",
		},
		{
			"we were told we have the wrong person",
			ThreadState{LastOutboundAt: ago(20), LastInboundAt: ago(21), BestIntent: IntentWrongPerson},
			"",
		},
		{
			"the only reply was a bounce, so there is nobody to chase",
			ThreadState{LastOutboundAt: ago(20), LastInboundAt: ago(20), LastKind: KindBounceHard},
			"",
		},
		{
			"the only reply was an autoresponder",
			ThreadState{LastOutboundAt: ago(20), LastInboundAt: ago(20), LastKind: KindAutoReplyOOO},
			"",
		},
		{
			"a platform notice is not a conversation",
			ThreadState{LastOutboundAt: ago(20), LastInboundAt: ago(19), LastKind: KindNotification},
			"",
		},
		{
			"nothing has been sent, so nobody is waiting on us",
			ThreadState{LastInboundAt: ago(5)},
			"",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FollowUp(tc.state, now); got != tc.want {
				t.Fatalf("FollowUp = %q, want %q", got, tc.want)
			}
		})
	}
}

// A clock that has gone backwards, or a message stamped in the future, must not
// produce a negative age and a label a day early.
func TestFollowUpToleratesFutureTimestamps(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	got := FollowUp(ThreadState{LastOutboundAt: now.AddDate(0, 0, 3)}, now)
	if got != "" {
		t.Fatalf("FollowUp with a future send = %q, want none", got)
	}
}

// Every state FollowUp can return has to be a label the workspace can filter
// on, or the feature produces something nobody can find.
func TestFollowUpLabelsAreAllCreated(t *testing.T) {
	all := map[string]bool{}
	for _, l := range AllLabels() {
		all[l] = true
	}
	for _, l := range FollowUpLabels {
		if !all[l] {
			t.Errorf("follow-up label %q is not in AllLabels, so it is never created and cannot be filtered on", l)
		}
	}
}

// ── Sweep ──────────────────────────────────────────────────────────────────

type fakeCategories struct {
	// labels[threadID] is what the thread currently wears.
	labels  map[string]map[string]bool
	seeded  []string
	removed map[string][]string
	// synced counts follow-up syncs per thread; onSync runs after each.
	synced map[string]int
	onSync func()
}

func (f *fakeCategories) EnsureCategory(_ context.Context, _ uuid.UUID, slug string) (uuid.UUID, error) {
	return uuid.NewSHA1(uuid.Nil, []byte(slug)), nil
}
func (f *fakeCategories) EnsureAll(_ context.Context, _ uuid.UUID, slugs []string) error {
	f.seeded = slugs
	return nil
}
func (f *fakeCategories) AddThreadLabels(_ context.Context, _ uuid.UUID, threadID string, ids []uuid.UUID) error {
	if f.labels == nil {
		f.labels = map[string]map[string]bool{}
	}
	if f.labels[threadID] == nil {
		f.labels[threadID] = map[string]bool{}
	}
	for _, id := range ids {
		f.labels[threadID][id.String()] = true
	}
	return nil
}
func (f *fakeCategories) SyncExclusiveLabels(ctx context.Context, orgID uuid.UUID, threadID string, family []string, want string) error {
	if f.labels == nil {
		f.labels = map[string]map[string]bool{}
	}
	if f.labels[threadID] == nil {
		f.labels[threadID] = map[string]bool{}
	}
	for _, slug := range family {
		if slug != want {
			delete(f.labels[threadID], slug)
		}
	}
	if want != "" {
		f.labels[threadID][want] = true
	}
	if f.synced == nil {
		f.synced = map[string]int{}
	}
	f.synced[threadID]++
	if f.onSync != nil {
		f.onSync()
	}
	return nil
}
func (f *fakeCategories) RemoveAutoLabels(_ context.Context, _ uuid.UUID, threadID string, slugs []string) error {
	if f.removed == nil {
		f.removed = map[string][]string{}
	}
	f.removed[threadID] = append(f.removed[threadID], slugs...)
	return nil
}
func (f *fakeCategories) has(threadID, label string) bool { return f.labels[threadID][label] }

// The sweep reuses stored classifications and makes no model calls of its own.
func TestSweepNeedsNoModel(t *testing.T) {
	now := time.Now()
	repo := &fakeRepo{states: []repository.ThreadFollowUpState{
		{ThreadID: "t-ours", LastOutboundAt: now.AddDate(0, 0, -6), LastInboundAt: now.AddDate(0, 0, -3), BestIntent: IntentWantsInfo, LastKind: KindHumanReply},
		{ThreadID: "t-chase", LastOutboundAt: now.AddDate(0, 0, -8)},
		{ThreadID: "t-cold", LastOutboundAt: now.AddDate(0, 0, -12), LastInboundAt: now.AddDate(0, 0, -14), BestIntent: IntentAgreed},
		{ThreadID: "t-closed", LastOutboundAt: now.AddDate(0, 0, -30), LastInboundAt: now.AddDate(0, 0, -31), BestIntent: IntentOptOut},
	}}
	cats := &fakeCategories{}
	asker := &countingAsker{}
	svc := NewService(asker, repo, cats, nil, true)

	p, err := svc.SweepFollowUps(context.Background(), uuid.New(), FollowUpSweep{Since: now.AddDate(0, 0, -90)})
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if asker.calls != 0 {
		t.Fatalf("the sweep made %d model calls; it is arithmetic over stored facts", asker.calls)
	}
	if p.Threads != 4 {
		t.Errorf("swept %d threads, want 4", p.Threads)
	}

	for _, tc := range []struct{ thread, label string }{
		{"t-ours", LabelNeedsReply},
		{"t-chase", LabelFollowUp},
		{"t-cold", LabelGoneQuiet},
	} {
		if !cats.has(tc.thread, tc.label) {
			t.Errorf("%s did not get %q", tc.thread, tc.label)
		}
	}
	if len(cats.labels["t-closed"]) != 0 {
		t.Errorf("an opted-out thread was labelled %v; it must never be chased", cats.labels["t-closed"])
	}
}

// The states move with the calendar, so a re-sweep has to REPLACE the label.
// A thread wearing both Follow up and Needs reply is wearing its history
// rather than its state.
func TestSweepReplacesRatherThanAccumulates(t *testing.T) {
	now := time.Now()
	cats := &fakeCategories{}
	repo := &fakeRepo{states: []repository.ThreadFollowUpState{
		{ThreadID: "t-1", LastOutboundAt: now.AddDate(0, 0, -9)},
	}}
	svc := NewService(&countingAsker{}, repo, cats, nil, true)
	orgID := uuid.New()

	if _, err := svc.SweepFollowUps(context.Background(), orgID, FollowUpSweep{Since: now.AddDate(0, 0, -90)}); err != nil {
		t.Fatalf("first sweep: %v", err)
	}
	if !cats.has("t-1", LabelFollowUp) {
		t.Fatal("expected Follow up on a send nobody answered")
	}

	// They answer, and we sit on it.
	repo.states[0].LastInboundAt = now.AddDate(0, 0, -3)
	repo.states[0].LastKind = KindHumanReply
	repo.states[0].BestIntent = IntentWantsInfo
	if _, err := svc.SweepFollowUps(context.Background(), orgID, FollowUpSweep{Since: now.AddDate(0, 0, -90)}); err != nil {
		t.Fatalf("second sweep: %v", err)
	}
	if !cats.has("t-1", LabelNeedsReply) {
		t.Error("did not move to Needs reply")
	}
	if cats.has("t-1", LabelFollowUp) {
		t.Error("kept Follow up alongside Needs reply; the thread now wears its history")
	}

	// We answer, so nothing is owed either way yet.
	repo.states[0].LastOutboundAt = now
	if _, err := svc.SweepFollowUps(context.Background(), orgID, FollowUpSweep{Since: now.AddDate(0, 0, -90)}); err != nil {
		t.Fatalf("third sweep: %v", err)
	}
	if cats.has("t-1", LabelNeedsReply) {
		t.Error("still asking us to reply after we did")
	}
}

// quietThreads is n threads we wrote to an hour ago, with one we wrote to
// eight days ago at index stale, which is due a Follow up.
func quietThreads(n, stale int) []repository.ThreadFollowUpState {
	now := time.Now()
	out := make([]repository.ThreadFollowUpState, n)
	for i := range out {
		out[i] = repository.ThreadFollowUpState{ThreadID: fmt.Sprintf("t-%d", i), LastOutboundAt: now.Add(-time.Hour)}
	}
	out[stale].LastOutboundAt = now.AddDate(0, 0, -8)
	return out
}

// A thread outside the newest 2,000 is reached: each pass takes one page and
// the next resumes after it, until the cycle ends and starts again at the newest.
func TestSweepPagesThroughEveryThread(t *testing.T) {
	repo := &fakeRepo{states: quietThreads(2500, 2400)}
	cats := &fakeCategories{}
	svc := NewService(&countingAsker{}, repo, cats, nil, true)
	orgID := uuid.New()
	opts := FollowUpSweep{Since: time.Now().AddDate(0, 0, -90), Budget: time.Nanosecond, PageSize: 500}

	passes := 0
	for {
		passes++
		p, err := svc.SweepFollowUps(context.Background(), orgID, opts)
		if err != nil {
			t.Fatalf("pass %d: %v", passes, err)
		}
		if p.Pages != 1 {
			t.Fatalf("pass %d read %d pages under a spent budget, want 1", passes, p.Pages)
		}
		if p.Complete {
			break
		}
		if repo.cursor == nil {
			t.Fatalf("pass %d stopped mid-cycle without saving where", passes)
		}
		if passes > 10 {
			t.Fatal("the cycle never completed")
		}
	}
	if passes != 6 {
		t.Errorf("took %d passes, want 6 (five full pages and the empty one that ends the cycle)", passes)
	}
	if !cats.has("t-2400", LabelFollowUp) {
		t.Error("the stale thread beyond the newest 2,000 was never labelled")
	}
	for i := range 2500 {
		if n := cats.synced[fmt.Sprintf("t-%d", i)]; n != 1 {
			t.Fatalf("t-%d was evaluated %d times in one cycle, want once", i, n)
		}
	}
	if repo.cursor != nil {
		t.Fatalf("a finished cycle left a cursor at %+v", repo.cursor)
	}

	// The next cycle starts again at the newest thread.
	if _, err := svc.SweepFollowUps(context.Background(), orgID, opts); err != nil {
		t.Fatalf("next cycle: %v", err)
	}
	if cats.synced["t-0"] != 2 || cats.synced["t-500"] != 1 {
		t.Errorf("next cycle evaluated t-0 %d and t-500 %d times, want it to restart at the newest page", cats.synced["t-0"], cats.synced["t-500"])
	}
}

// A pass cut off mid-page keeps the cursor on the last thread it evaluated, and
// the next pass picks up right after it.
func TestSweepResumesAfterTheLastEvaluatedThread(t *testing.T) {
	repo := &fakeRepo{states: quietThreads(1200, 1100)}
	cats := &fakeCategories{}
	svc := NewService(&countingAsker{}, repo, cats, nil, true)
	orgID := uuid.New()
	opts := FollowUpSweep{Since: time.Now().AddDate(0, 0, -90), PageSize: 500}

	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	cats.onSync = func() {
		if calls++; calls == 700 {
			cancel()
		}
	}
	if _, err := svc.SweepFollowUps(ctx, orgID, opts); !errors.Is(err, context.Canceled) {
		t.Fatalf("interrupted sweep returned %v, want context.Canceled", err)
	}
	if repo.cursor == nil || repo.cursor.RowID != repo.position(699).RowID {
		t.Fatalf("cursor %+v, want the 700th thread, the last one evaluated", repo.cursor)
	}

	cats.onSync = nil
	p, err := svc.SweepFollowUps(context.Background(), orgID, opts)
	if err != nil {
		t.Fatalf("resumed sweep: %v", err)
	}
	if p.Threads != 500 || !p.Complete {
		t.Fatalf("resumed sweep evaluated %d threads (complete %v), want the 500 left", p.Threads, p.Complete)
	}
	for i := range 1200 {
		if n := cats.synced[fmt.Sprintf("t-%d", i)]; n != 1 {
			t.Fatalf("t-%d evaluated %d times across the interrupted and resumed passes, want once", i, n)
		}
	}
	if !cats.has("t-1100", LabelFollowUp) {
		t.Error("the stale thread after the interruption was never labelled")
	}
}

// A reply mid-cycle is swept in the next pass, not when the cycle comes back round.
func TestSweepTakesFreshThreadsFirstWhenResuming(t *testing.T) {
	now := time.Now()
	repo := &fakeRepo{states: quietThreads(3000, 2999), base: now}
	// They answered an hour ago in a thread we chased a week ago.
	repo.states[60] = repository.ThreadFollowUpState{ThreadID: "t-60", LastOutboundAt: now.AddDate(0, 0, -7), LastInboundAt: now.Add(-time.Hour), BestIntent: IntentWantsInfo, LastKind: KindHumanReply}
	cats := &fakeCategories{labels: map[string]map[string]bool{"t-60": {LabelFollowUp: true}}}
	pos := repo.position(1000)
	repo.cursor = &pos
	svc := NewService(&countingAsker{}, repo, cats, nil, true)

	p, err := svc.SweepFollowUps(context.Background(), uuid.New(), FollowUpSweep{
		Since: now.AddDate(0, 0, -90), Fresh: 3*time.Hour + 30*time.Second, Budget: time.Nanosecond, PageSize: 500,
	})
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if cats.has("t-60", LabelFollowUp) {
		t.Error("a thread they answered still wears Follow up while the cycle is elsewhere")
	}
	if cats.synced["t-1001"] != 1 {
		t.Error("the cycle did not move on from its cursor in the same pass")
	}
	if p.Threads != 181+500 {
		t.Errorf("swept %d threads, want the 181 fresh ones and one cycle page", p.Threads)
	}
}

// An operator's full run neither reads nor moves the hourly sweep's cursor.
func TestFullSweepLeavesTheCursorAlone(t *testing.T) {
	repo := &fakeRepo{states: quietThreads(1200, 1100)}
	pos := repo.position(600)
	repo.cursor = &pos
	cats := &fakeCategories{}
	svc := NewService(&countingAsker{}, repo, cats, nil, true)

	p, err := svc.SweepFollowUps(context.Background(), uuid.New(), FollowUpSweep{Since: time.Now().AddDate(0, 0, -90), Full: true})
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if p.Threads != 1200 || !p.Complete {
		t.Fatalf("full sweep evaluated %d threads (complete %v), want all 1200", p.Threads, p.Complete)
	}
	if repo.cursor == nil || repo.cursor.RowID != pos.RowID {
		t.Fatalf("full sweep moved the cursor to %+v", repo.cursor)
	}
}
