package repository

import (
	"context"
	"testing"
	"time"

	"github.com/warmbly/warmbly/internal/models"
)

// A withdrawn tampering strike revises the hold it imposed wherever that hold
// lives: the pool row, or the ledger of a mailbox out of every pool, which
// would otherwise seed the hold back on rejoin.
//
//	WARMBLY_TEST_DB=postgres://warmbly:warmbly@localhost:15432/warmbly_x?sslmode=disable \
//	  go test ./internal/repository/ -run LiveTamperingHold -v
func TestLiveTamperingHoldRevision(t *testing.T) {
	f := newLedgerFixture(t)
	ctx := context.Background()
	id := f.addMailbox(t, f.user)
	f.join(t, id)
	until := time.Now().Add(6 * 24 * time.Hour)
	f.exec(t, `UPDATE warmup_pool_participants SET health_state = 'quarantined', blocked_at = now(),
	           blocked_until = $2, blocked_reason = 'Paused from warmup: x' WHERE email_account_id = $1`, id, until)

	hold, err := f.warmups.GetWarmupHold(ctx, id)
	if err != nil || hold == nil || !hold.InPool || hold.State != models.WarmupHealthQuarantined || hold.Reason != "Paused from warmup: x" {
		t.Fatalf("pool hold = %+v, %v", hold, err)
	}
	stale := *hold
	moved := hold.BlockedUntil.Add(time.Hour)
	stale.BlockedUntil = &moved
	if ok, err := f.warmups.ReviseWarmupHold(ctx, id, &stale, models.WarmupHealthHealthy, nil, ""); err != nil || ok {
		t.Fatalf("revised a hold whose term changed since it was read: %v %v", ok, err)
	}

	// Out of every pool, the ledger speaks for the mailbox.
	if err := f.warmups.LeaveAllPools(ctx, id); err != nil {
		t.Fatal(err)
	}
	hold, err = f.warmups.GetWarmupHold(ctx, id)
	if err != nil || hold == nil || hold.InPool || hold.State != models.WarmupHealthQuarantined {
		t.Fatalf("ledger hold = %+v, %v", hold, err)
	}
	shorter := time.Now().Add(time.Hour)
	if ok, err := f.warmups.ReviseWarmupHold(ctx, id, hold, models.WarmupHealthQuarantined, &shorter, "Paused from warmup: y"); err != nil || !ok {
		t.Fatalf("ledger revision: %v %v", ok, err)
	}
	if hold, err = f.warmups.GetWarmupHold(ctx, id); err != nil || hold == nil || hold.Reason != "Paused from warmup: y" {
		t.Fatalf("ledger after revision = %+v, %v", hold, err)
	}
	if ok, err := f.warmups.ReviseWarmupHold(ctx, id, hold, models.WarmupHealthHealthy, nil, ""); err != nil || !ok {
		t.Fatalf("ledger lift: %v %v", ok, err)
	}
	if hold, err = f.warmups.GetWarmupHold(ctx, id); err != nil || hold != nil {
		t.Fatalf("ledger still holds %+v, %v", hold, err)
	}

	// Rejoining seeds nothing, and a pool-row lift clears the ledger mirror too.
	f.join(t, id)
	f.exec(t, `UPDATE warmup_pool_participants SET health_state = 'blocked', blocked_at = now(),
	           blocked_until = $2, blocked_reason = 'Blocked from warmup: x' WHERE email_account_id = $1`, id, until)
	if hold, err = f.warmups.GetWarmupHold(ctx, id); err != nil || hold == nil || !hold.InPool {
		t.Fatalf("pool hold = %+v, %v", hold, err)
	}
	if ok, err := f.warmups.ReviseWarmupHold(ctx, id, hold, models.WarmupHealthHealthy, nil, ""); err != nil || !ok {
		t.Fatalf("pool lift: %v %v", ok, err)
	}
	var ledger int
	if err := f.pool.QueryRow(ctx, `SELECT COUNT(*) FROM warmup_reputation_ledger WHERE organization_id = $1`, f.org).Scan(&ledger); err != nil || ledger != 0 {
		t.Fatalf("ledger rows after lift = %d, %v", ledger, err)
	}
	if hold, err = f.warmups.GetWarmupHold(ctx, id); err != nil || hold == nil || hold.State != models.WarmupHealthHealthy || hold.BlockedUntil != nil {
		t.Fatalf("pool row after lift = %+v, %v", hold, err)
	}
}

// Strikes from before the search are listed until one is asked for, then
// again after the retry window; confirmed and new strikes never are.
func TestLiveTamperingUnverifiedListing(t *testing.T) {
	f := newLedgerFixture(t)
	ctx := context.Background()
	id := f.addMailbox(t, f.user)
	worker := f.org // any uuid; the listing only needs one set
	f.exec(t, `INSERT INTO fleet_nodes (id, role, name) VALUES ($1, 'worker', 'tamper-test') ON CONFLICT DO NOTHING`, worker)
	f.exec(t, `INSERT INTO workers (id) VALUES ($1) ON CONFLICT DO NOTHING`, worker)
	f.exec(t, `UPDATE email_accounts SET worker_id = $2 WHERE id = $1`, id, worker)
	t.Cleanup(func() {
		_, _ = f.pool.Exec(ctx, `DELETE FROM warmup_tampering_events WHERE email_account_id = $1`, id)
		_, _ = f.pool.Exec(ctx, `UPDATE email_accounts SET worker_id = NULL WHERE id = $1`, id)
		_, _ = f.pool.Exec(ctx, `DELETE FROM workers WHERE id = $1`, worker)
		_, _ = f.pool.Exec(ctx, `DELETE FROM fleet_nodes WHERE id = $1`, worker)
	})
	f.exec(t, `INSERT INTO warmup_tampering_events (email_account_id, message_id, kind, verified_at) VALUES ($1, '<old@t>', 'deletion', NULL)`, id)
	if _, err := f.warmups.RecordWarmupTampering(ctx, id, "<new@t>", "deletion"); err != nil {
		t.Fatal(err)
	}

	list := func() []WarmupTamperingToVerify {
		rows, err := f.warmups.ListUnverifiedDeletions(ctx, time.Now().Add(-time.Hour), 6*time.Hour, 10)
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}
	rows := list()
	if len(rows) != 1 || rows[0].MessageID != "<old@t>" || rows[0].WorkerID != worker || rows[0].UserID != f.user {
		t.Fatalf("listed %+v, want only the pre-search strike", rows)
	}
	if err := f.warmups.MarkTamperingVerifyRequested(ctx, id, "<old@t>"); err != nil {
		t.Fatal(err)
	}
	if rows = list(); len(rows) != 0 {
		t.Fatalf("listed %+v right after asking", rows)
	}
	f.exec(t, `UPDATE warmup_tampering_events SET verify_requested_at = now() - interval '7 hours' WHERE email_account_id = $1`, id)
	if rows = list(); len(rows) != 1 {
		t.Fatalf("an unanswered search was not offered again: %+v", rows)
	}
	if err := f.warmups.MarkTamperingVerified(ctx, id, "<old@t>", "deletion"); err != nil {
		t.Fatal(err)
	}
	if rows = list(); len(rows) != 0 {
		t.Fatalf("listed a confirmed strike: %+v", rows)
	}
	d, s, err := f.warmups.CountWarmupTamperingBetween(ctx, id, time.Now().Add(-time.Hour), time.Now().Add(time.Minute), "<new@t>", false)
	if err != nil || d != 1 || s != 0 {
		t.Fatalf("counted %d deletions, %d flags excluding one, %v", d, s, err)
	}
	// A second mailbox on the same address counts only by address.
	sib := f.addMailbox(t, f.user)
	t.Cleanup(func() {
		_, _ = f.pool.Exec(ctx, `DELETE FROM warmup_tampering_events WHERE email_account_id = $1`, sib)
	})
	if _, err := f.warmups.RecordWarmupTampering(ctx, sib, "<sib@t>", "spam_flag"); err != nil {
		t.Fatal(err)
	}
	if _, s, _ = f.warmups.CountWarmupTamperingBetween(ctx, id, time.Now().Add(-time.Hour), time.Now().Add(time.Minute), "", false); s != 0 {
		t.Fatalf("counted a sibling's flag for the mailbox alone")
	}
	if _, s, _ = f.warmups.CountWarmupTamperingBetween(ctx, id, time.Now().Add(-time.Hour), time.Now().Add(time.Minute), "", true); s != 1 {
		t.Fatalf("by address counted %d flags, want the sibling's", s)
	}
	if retired, err := f.warmups.WarmupReceiptRetired(ctx, id, "<old@t>"); err != nil || retired {
		t.Fatalf("retired = %v, %v with no receipt", retired, err)
	}
}
