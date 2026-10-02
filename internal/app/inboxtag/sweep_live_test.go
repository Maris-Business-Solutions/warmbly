package inboxtag

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
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
	if err := pool.QueryRow(ctx, `SELECT version FROM schema_migrations LIMIT 1`).Scan(&version); err != nil || version < 244 {
		t.Fatalf("WARMBLY_TEST_DB is at schema version %d (err %v); this test needs 244 or later", version, err)
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
	repo := repository.NewInboxTagRepository(f.pool)
	svc := NewService(nil, repo, repository.NewTagCategoryStore(f.pool), nil, true)
	p, err := svc.SweepFollowUps(context.Background(), f.org, FollowUpSweep{
		Since: time.Now().AddDate(0, 0, -90), Budget: time.Nanosecond, PageSize: 500,
	})
	if err != nil {
		f.t.Fatalf("sweep: %v", err)
	}
	return p
}

func (f *sweepFixture) cursor() *repository.FollowUpPosition {
	f.t.Helper()
	pos, err := repository.NewInboxTagRepository(f.pool).FollowUpCursor(context.Background(), f.org)
	if err != nil {
		f.t.Fatalf("cursor: %v", err)
	}
	return pos
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
