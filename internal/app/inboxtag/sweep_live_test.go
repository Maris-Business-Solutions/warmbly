package inboxtag

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/warmbly/warmbly/internal/repository"
)

// The follow-up sweep against Postgres, with more threads than one old-style
// pass ever reached. Skipped unless WARMBLY_TEST_DB is set:
//
//	WARMBLY_TEST_DB=postgres://warmbly:warmbly@localhost:15432/<db>?sslmode=disable \
//	  go test ./internal/app/inboxtag/ -run LiveFollowUpSweep -v

const liveBusyThreads = 2100

type sweepFixture struct {
	t       *testing.T
	pool    *pgxpool.Pool
	owner   uuid.UUID
	org     uuid.UUID
	mailbox [2]uuid.UUID
	store   *repository.TagCategoryStore
}

func newSweepFixture(t *testing.T) *sweepFixture {
	t.Helper()
	dsn := os.Getenv("WARMBLY_TEST_DB")
	if dsn == "" {
		t.Skip("WARMBLY_TEST_DB not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	var version int64
	if err := pool.QueryRow(ctx, `SELECT version FROM schema_migrations LIMIT 1`).Scan(&version); err != nil || version < 249 {
		t.Fatalf("WARMBLY_TEST_DB is at schema version %d (err %v); this test needs 249 or later", version, err)
	}

	f := &sweepFixture{t: t, pool: pool, owner: uuid.New(), org: uuid.New(), mailbox: [2]uuid.UUID{uuid.New(), uuid.New()}}
	f.store = repository.NewTagCategoryStore(pool)
	f.exec(`INSERT INTO users (id, first_name, last_name, email, password_hash) VALUES ($1, 'Sweep', 'Owner', $2, 'x')`,
		f.owner, "sweep-"+f.owner.String()[:8]+"@test.local")
	f.exec(`INSERT INTO organizations (id, name, slug, owner_user_id) VALUES ($1, 'Sweep', $2, $3)`, f.org, "sweep-"+f.org.String()[:8], f.owner)
	for _, mb := range f.mailbox {
		f.exec(`INSERT INTO email_accounts (id, user_id, organization_id, email, name,
		            signature_plain, signature_html, provider, status, campaign_limit, min_wait_time, timezone)
		        VALUES ($1, $2, $3, $4, 'Sweep', '', '', 'smtp_imap', 'active', 50, 0, 'UTC')`,
			mb, f.owner, f.org, "sweep-mb-"+mb.String()[:8]+"@test.local")
	}
	t.Cleanup(func() {
		c := context.Background()
		for _, sql := range []string{
			`DELETE FROM unibox_thread_labels WHERE organization_id = $1`,
			`DELETE FROM categories WHERE organization_id = $1`,
			`DELETE FROM inbox_tag_results WHERE organization_id = $1`,
			`DELETE FROM unibox_emails WHERE email_id IN (SELECT id FROM email_accounts WHERE organization_id = $1)`,
			`DELETE FROM inbox_follow_up_sweeps WHERE organization_id = $1`,
			`DELETE FROM email_accounts WHERE organization_id = $1`,
			`DELETE FROM organizations WHERE id = $1`,
		} {
			if _, err := pool.Exec(c, sql, f.org); err != nil {
				t.Errorf("cleanup %q: %v", sql, err)
			}
		}
		if _, err := pool.Exec(c, `DELETE FROM users WHERE id = $1`, f.owner); err != nil {
			t.Errorf("cleanup user: %v", err)
		}
	})

	// The newest 2,100 threads: we wrote an hour ago, nothing is owed yet.
	f.exec(`INSERT INTO unibox_emails (id, user_id, email_id, folder, provider_folder, message_id, thread_id,
	            from_addr, subject, body_text, internal_date)
	        SELECT gen_random_uuid(), $1, CASE WHEN g % 2 = 0 THEN $2::uuid ELSE $3::uuid END, 'sent', 'sent',
	               '<busy-' || g || '@sweep.test>', 'busy-' || g, ARRAY['me@sweep.test'], 'Hello', 'body',
	               NOW() - interval '1 hour' - g * interval '1 second'
	        FROM generate_series(1, $4::int) g`, f.owner, f.mailbox[0], f.mailbox[1], liveBusyThreads)

	day := func(n int) time.Time { return time.Now().AddDate(0, 0, -n) }
	// Every conversation below is older than all of those.
	f.message(0, "sent", "stale-chase", day(8), "", "")
	f.message(0, "sent", "stale-cold", day(12), "", "")
	f.message(0, "inbox", "stale-cold", day(14), KindHumanReply, IntentAgreed)
	f.message(1, "sent", "stale-owed", day(9), "", "")
	f.message(1, "inbox", "stale-owed", day(4), KindHumanReply, IntentWantsInfo)
	f.message(0, "sent", "stale-declined", day(20), "", "")
	f.message(0, "inbox", "stale-declined", day(19), KindHumanReply, IntentNotInterested)
	f.message(1, "sent", "stale-bounce", day(20), "", "")
	f.message(1, "inbox", "stale-bounce", day(19), KindBounceHard, "")
	f.message(0, "sent", "stale-answered", day(10), "", "")
	f.message(0, "inbox", "stale-answered", day(1), KindHumanReply, IntentWantsInfo)
	f.label("stale-answered", LabelFollowUp)
	// One conversation held by both mailboxes is still one thread.
	f.message(1, "inbox", "stale-shared", day(9), "", "")
	f.message(0, "sent", "stale-shared", day(8), "", "")
	return f
}

// liveThreads is every thread the fixture has that we wrote to.
const liveThreads = liveBusyThreads + 7

func (f *sweepFixture) exec(sql string, args ...any) {
	f.t.Helper()
	if _, err := f.pool.Exec(context.Background(), sql, args...); err != nil {
		f.t.Fatalf("fixture %q: %v", sql[:min(70, len(sql))], err)
	}
}

func (f *sweepFixture) message(mailbox int, folder, thread string, at time.Time, kind, intent string) {
	f.t.Helper()
	id := fmt.Sprintf("<%s-%s-%d@sweep.test>", thread, folder, mailbox)
	f.exec(`INSERT INTO unibox_emails (id, user_id, email_id, folder, provider_folder, message_id, thread_id,
	            from_addr, subject, body_text, internal_date)
	        VALUES ($1, $2, $3, $4, $4, $5, $6, ARRAY['x@sweep.test'], 'Hello', 'body', $7)`,
		uuid.New(), f.owner, f.mailbox[mailbox], folder, id, thread, at)
	if kind != "" {
		f.exec(`INSERT INTO inbox_tag_results (organization_id, email_account_id, message_id, thread_id, status, kind, intent)
		        VALUES ($1, $2, $3, $4, 'complete', $5, $6)`, f.org, f.mailbox[mailbox], id, thread, kind, intent)
	}
}

func (f *sweepFixture) label(thread, slug string) {
	f.t.Helper()
	id, err := f.store.EnsureCategory(context.Background(), f.org, slug)
	if err != nil {
		f.t.Fatalf("category: %v", err)
	}
	f.exec(`INSERT INTO unibox_thread_labels (organization_id, thread_id, category_id) VALUES ($1, $2, $3)`, f.org, thread, id)
}

func (f *sweepFixture) labels(thread string) []string {
	f.t.Helper()
	rows, err := f.pool.Query(context.Background(), `
		SELECT c.title FROM unibox_thread_labels l JOIN categories c ON c.id = l.category_id
		WHERE l.organization_id = $1 AND l.thread_id = $2 ORDER BY c.title`, f.org, thread)
	if err != nil {
		f.t.Fatalf("labels: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			f.t.Fatalf("scan: %v", err)
		}
		out = append(out, title)
	}
	return out
}

// pass is one hourly pass from a freshly started service, as after a consumer restart.
func (f *sweepFixture) pass() FollowUpProgress {
	f.t.Helper()
	return f.passWith(FollowUpSweep{Since: time.Now().AddDate(0, 0, -90), Budget: time.Nanosecond, PageSize: 500})
}

func (f *sweepFixture) passWith(opts FollowUpSweep) FollowUpProgress {
	f.t.Helper()
	repo := repository.NewInboxTagRepository(f.pool)
	svc := NewService(nil, repo, repository.NewTagCategoryStore(f.pool), nil, true)
	p, err := svc.SweepFollowUps(context.Background(), f.org, opts)
	if err != nil {
		f.t.Fatalf("sweep: %v", err)
	}
	return p
}

func (f *sweepFixture) cursor() *repository.FollowUpPosition {
	f.t.Helper()
	var mailbox, row *uuid.UUID
	var at *time.Time
	err := f.pool.QueryRow(context.Background(), `
		SELECT email_account_id, internal_date, message_row_id FROM inbox_follow_up_sweeps WHERE organization_id = $1`,
		f.org).Scan(&mailbox, &at, &row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		f.t.Fatalf("cursor: %v", err)
	}
	if mailbox == nil || at == nil || row == nil {
		return nil
	}
	return &repository.FollowUpPosition{MailboxID: *mailbox, At: *at, RowID: *row}
}

// A conversation older than the newest 2,000 is reached within one cycle and
// gets the label its calendar says, with automated and declined threads left
// alone and a stale label taken off.
func TestLiveFollowUpSweepReachesThreadsBeyondTheNewest2000(t *testing.T) {
	f := newSweepFixture(t)

	threads := 0
	for passes := 1; ; passes++ {
		p := f.pass()
		threads += p.Threads
		if p.Complete {
			break
		}
		if passes > 20 {
			t.Fatal("the cycle never completed")
		}
	}
	if threads != liveThreads {
		t.Errorf("a cycle evaluated %d threads, want each of the %d once", threads, liveThreads)
	}

	for thread, want := range map[string]string{
		"stale-chase":    LabelFollowUp,
		"stale-cold":     LabelGoneQuiet,
		"stale-owed":     LabelNeedsReply,
		"stale-declined": "",
		"stale-bounce":   "",
		"stale-answered": "",
		"stale-shared":   LabelFollowUp,
		"busy-1":         "",
	} {
		got := f.labels(thread)
		switch {
		case want == "" && len(got) != 0:
			t.Errorf("%s wears %v, want nothing", thread, got)
		case want != "" && (len(got) != 1 || got[0] != want):
			t.Errorf("%s wears %v, want %q", thread, got, want)
		}
	}
	if pos := f.cursor(); pos != nil {
		t.Errorf("a finished cycle left a cursor at %+v", pos)
	}
}

// A pass that stops resumes where it left off from the saved cursor, so each
// thread is evaluated once per cycle rather than the newest page every time.
func TestLiveFollowUpSweepResumesFromItsCursor(t *testing.T) {
	f := newSweepFixture(t)

	first := f.pass()
	if first.Complete || first.Threads == 0 {
		t.Fatalf("first pass %+v, want one partial page", first)
	}
	prev := f.cursor()
	if prev == nil {
		t.Fatal("a pass stopped mid-cycle without saving where")
	}

	threads := first.Threads
	for passes := 2; ; passes++ {
		p := f.pass()
		threads += p.Threads
		if p.Complete {
			break
		}
		pos := f.cursor()
		if pos == nil || !walkedPast(*pos, *prev) {
			t.Fatalf("pass %d left the cursor at %+v, not past %+v", passes, pos, prev)
		}
		prev = pos
		if passes > 20 {
			t.Fatal("the cycle never completed")
		}
	}
	if threads != liveThreads {
		t.Errorf("the resumed passes evaluated %d threads, want each of the %d once", threads, liveThreads)
	}
	if got := f.labels("stale-chase"); len(got) != 1 || got[0] != LabelFollowUp {
		t.Errorf("stale-chase wears %v after a resumed cycle", got)
	}
}

// walkedPast reports whether a comes after b in the sweep's walk order.
func walkedPast(a, b repository.FollowUpPosition) bool {
	if c := bytes.Compare(a.MailboxID[:], b.MailboxID[:]); c != 0 {
		return c > 0
	}
	if !a.At.Equal(b.At) {
		return a.At.Before(b.At)
	}
	return bytes.Compare(a.RowID[:], b.RowID[:]) < 0
}

// A reply the sync stored late, under a date older than any fixed window, and
// a verdict written long after its message, are both checked in the next pass
// while the cycle is still among the newest threads.
func TestLiveFollowUpSweepChecksLateSyncedReplies(t *testing.T) {
	f := newSweepFixture(t)
	ctx := context.Background()
	// Everything the fixture wrote was stored and judged two days ago.
	f.exec(`UPDATE unibox_emails SET ingested_at = NOW() - interval '2 days'
	        WHERE email_id IN (SELECT id FROM email_accounts WHERE organization_id = $1)`, f.org)
	f.exec(`UPDATE inbox_tag_results SET updated_at = NOW() - interval '2 days' WHERE organization_id = $1`, f.org)
	f.label("stale-chase", LabelFollowUp)
	// Inbound four days ago, after our send, not judged yet: nothing is owed until it is.
	f.message(1, "sent", "late-verdict", time.Now().AddDate(0, 0, -9), "", "")
	f.message(1, "inbox", "late-verdict", time.Now().AddDate(0, 0, -4), "", "")
	f.exec(`UPDATE unibox_emails SET ingested_at = NOW() - interval '2 days' WHERE thread_id = 'late-verdict'`)

	opts := FollowUpSweep{Since: time.Now().AddDate(0, 0, -90), Fresh: 24 * time.Hour, Budget: time.Nanosecond, PageSize: 500}
	if p := f.passWith(opts); p.Complete {
		t.Fatalf("first pass finished the cycle; the test needs it elsewhere: %+v", p)
	}
	if got := f.labels("late-verdict"); len(got) != 0 {
		t.Fatalf("late-verdict wears %v before its reply was judged", got)
	}
	// As if that pass ran ten minutes ago.
	f.exec(`UPDATE inbox_follow_up_sweeps SET fresh_at = fresh_at - interval '10 minutes' WHERE organization_id = $1`, f.org)

	// They answered six hours ago; the sync stored it five minutes ago.
	id := "<late-reply@sweep.test>"
	f.exec(`INSERT INTO unibox_emails (id, user_id, email_id, folder, provider_folder, message_id, thread_id,
	            from_addr, subject, body_text, internal_date, ingested_at)
	        VALUES ($1, $2, $3, 'inbox', 'inbox', $4, 'stale-chase', ARRAY['x@sweep.test'], 'Re: Hello', 'body',
	                NOW() - interval '6 hours', NOW() - interval '5 minutes')`, uuid.New(), f.owner, f.mailbox[0], id)
	f.exec(`INSERT INTO inbox_tag_results (organization_id, email_account_id, message_id, thread_id, status, kind, intent, updated_at)
	        VALUES ($1, $2, $3, 'stale-chase', 'complete', $4, $5, NOW() - interval '2 days')`,
		f.org, f.mailbox[0], id, KindHumanReply, IntentWantsInfo)
	// The four-day-old reply is judged five minutes ago.
	f.exec(`INSERT INTO inbox_tag_results (organization_id, email_account_id, message_id, thread_id, status, kind, intent, updated_at)
	        VALUES ($1, $2, '<late-verdict-inbox-1@sweep.test>', 'late-verdict', 'complete', $3, $4, NOW() - interval '5 minutes')`,
		f.org, f.mailbox[1], KindHumanReply, IntentWantsInfo)

	before := f.cursor()
	f.passWith(opts)
	if got := f.labels("stale-chase"); len(got) != 0 {
		t.Errorf("stale-chase still wears %v after a reply stored since the last pass", got)
	}
	if got := f.labels("late-verdict"); len(got) != 1 || got[0] != LabelNeedsReply {
		t.Errorf("late-verdict wears %v after its reply was judged, want Needs reply", got)
	}
	if after := f.cursor(); after == nil || before == nil || after.MailboxID != before.MailboxID {
		t.Fatalf("the cycle left the first mailbox (%+v -> %+v); the threads may have been reached by it", before, after)
	}
	var freshAt time.Time
	if err := f.pool.QueryRow(ctx, `SELECT fresh_at FROM inbox_follow_up_sweeps WHERE organization_id = $1`, f.org).Scan(&freshAt); err != nil {
		t.Fatalf("fresh_at: %v", err)
	}
	if time.Since(freshAt) > 3*time.Minute {
		t.Errorf("the changed-thread mark stayed at %v", freshAt)
	}
}

// While one walker holds a workspace, another neither sweeps it nor moves its state.
func TestLiveFollowUpSweepLeaseKeepsOneWalker(t *testing.T) {
	f := newSweepFixture(t)
	ctx := context.Background()
	f.pass()
	before := f.cursor()
	if before == nil {
		t.Fatal("first pass saved no cursor")
	}

	repo := repository.NewInboxTagRepository(f.pool)
	holder := uuid.New()
	st, err := repo.ClaimFollowUpSweep(ctx, f.org, holder, time.Minute)
	if err != nil || st == nil {
		t.Fatalf("claim: %+v %v", st, err)
	}
	if p := f.pass(); !p.Busy || p.Threads != 0 {
		t.Fatalf("a second walker swept a leased workspace: %+v", p)
	}
	if again, err := repo.ClaimFollowUpSweep(ctx, f.org, uuid.New(), time.Minute); err != nil || again != nil {
		t.Fatalf("a second claim succeeded: %+v %v", again, err)
	}
	if ok, err := repo.SaveFollowUpSweep(ctx, f.org, uuid.New(), time.Minute, repository.FollowUpSweepState{}); err != nil || ok {
		t.Fatalf("a walker without the lease saved: %v %v", ok, err)
	}
	if got := f.cursor(); got == nil || !got.At.Equal(before.At) || got.RowID != before.RowID {
		t.Fatalf("the cursor moved to %+v while another walker held the lease", got)
	}
	if ok, err := repo.SaveFollowUpSweep(ctx, f.org, holder, time.Minute, *st); err != nil || !ok {
		t.Fatalf("the holder could not save: %v %v", ok, err)
	}
	if err := repo.ReleaseFollowUpSweep(ctx, f.org, holder); err != nil {
		t.Fatalf("release: %v", err)
	}
	if p := f.pass(); p.Busy || p.Threads == 0 {
		t.Fatalf("the released workspace was not swept: %+v", p)
	}
}

// A walker that outlives its lease keeps saving until another walker takes the workspace over.
func TestLiveFollowUpSweepLeaseOverrun(t *testing.T) {
	f := newSweepFixture(t)
	ctx := context.Background()
	repo := repository.NewInboxTagRepository(f.pool)

	slow := uuid.New()
	st, err := repo.ClaimFollowUpSweep(ctx, f.org, slow, time.Millisecond)
	if err != nil || st == nil {
		t.Fatalf("claim: %+v %v", st, err)
	}
	time.Sleep(20 * time.Millisecond)
	if ok, err := repo.SaveFollowUpSweep(ctx, f.org, slow, time.Millisecond, *st); err != nil || !ok {
		t.Fatalf("the holder could not save after its lease lapsed: %v %v", ok, err)
	}
	time.Sleep(20 * time.Millisecond)
	next := uuid.New()
	taken, err := repo.ClaimFollowUpSweep(ctx, f.org, next, time.Minute)
	if err != nil || taken == nil {
		t.Fatalf("a lapsed lease could not be taken over: %+v %v", taken, err)
	}
	if ok, err := repo.SaveFollowUpSweep(ctx, f.org, slow, time.Minute, *st); err != nil || ok {
		t.Fatalf("the old holder saved after a takeover: %v %v", ok, err)
	}
	if ok, err := repo.SaveFollowUpSweep(ctx, f.org, next, time.Minute, *taken); err != nil || !ok {
		t.Fatalf("the new holder could not save: %v %v", ok, err)
	}
}
