package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// A refused warmup send is reported until a later send goes out.
func TestLiveLastWarmupSendFailure(t *testing.T) {
	_, pool := liveContactDB(t)
	ctx := context.Background()
	f := newWarmupUsageFixture(t, pool)
	repo := &warmupRepository{db: pool}
	since := time.Now().Add(-24 * time.Hour)

	failed := func(at time.Time, message string) {
		id := uuid.New()
		f.exec(`INSERT INTO tasks (id, task_type, email_account_id, status, message_id, updated_at)
		        VALUES ($1, 'warmup', $2, 'failed', '', $3)`, id, f.account, at)
		f.exec(`INSERT INTO task_failures (task_id, title, message) VALUES ($1, 'Send failed', $2)`, id, message)
	}

	if got, err := repo.LastWarmupSendFailure(ctx, f.account, since); err != nil || got != nil {
		t.Fatalf("no failures: got %+v, %v; want nil", got, err)
	}

	failed(time.Now().Add(-3*time.Hour), "older refusal")
	failed(time.Now().Add(-2*time.Hour), "dial smtp.example.test:465: connection refused")
	got, err := repo.LastWarmupSendFailure(ctx, f.account, since)
	if err != nil || got == nil || got.Message != "dial smtp.example.test:465: connection refused" {
		t.Fatalf("got %+v, %v; want the newest refusal", got, err)
	}

	// A dead-lettered dispatch is the platform's, not the server's answer.
	dead := uuid.New()
	f.exec(`INSERT INTO tasks (id, task_type, email_account_id, status, message_id, updated_at)
	        VALUES ($1, 'warmup', $2, 'dead_lettered', '', NOW())`, dead, f.account)
	f.exec(`INSERT INTO task_failures (task_id, title, message) VALUES ($1, 'Send failed', 'worker offline')`, dead)
	if got, _ := repo.LastWarmupSendFailure(ctx, f.account, since); got == nil || got.Message == "worker offline" {
		t.Fatalf("got %+v; a dead-lettered task must not be reported", got)
	}

	if got, _ := repo.LastWarmupSendFailure(ctx, f.account, time.Now().Add(-time.Hour)); got != nil {
		t.Fatalf("got %+v; a refusal before the window must not be reported", got)
	}

	// A send that went out after it clears the report.
	f.exec(`INSERT INTO tasks (id, task_type, email_account_id, status, message_id, completed_at)
	        VALUES ($1, 'warmup', $2, 'completed', '', NOW() - interval '1 hour')`, uuid.New(), f.account)
	if got, err := repo.LastWarmupSendFailure(ctx, f.account, since); err != nil || got != nil {
		t.Fatalf("after a delivered send: got %+v, %v; want nil", got, err)
	}
}
