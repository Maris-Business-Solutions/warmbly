package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/models"
)

// The mailbox-limited reads see only the named mailboxes, and MessageMailboxes
// names the mailbox behind a message and a conversation.
func TestLiveUniboxMailboxScopedReads(t *testing.T) {
	handle := liveUniboxFolderDB(t)
	f := newUniboxFolderFixture(t, handle.Pool)
	repo := NewUniboxRepository(handle)
	ctx := context.Background()
	now := time.Now().UTC()

	id := f.scopedMessage(t, repo, "thread-scope", "them@example.com", models.FolderInbox, now)

	boxes, err := repo.MessageMailboxes(ctx, f.org, []uuid.UUID{id}, nil)
	if err != nil || len(boxes) != 1 || boxes[0] != f.mailbox {
		t.Fatalf("MessageMailboxes by id = %v, %v", boxes, err)
	}
	boxes, err = repo.MessageMailboxes(ctx, f.org, nil, []string{"thread-scope"})
	if err != nil || len(boxes) != 1 || boxes[0] != f.mailbox {
		t.Fatalf("MessageMailboxes by thread = %v, %v", boxes, err)
	}
	if boxes, err = repo.MessageMailboxes(ctx, uuid.New(), []uuid.UUID{id}, []string{"thread-scope"}); err != nil || len(boxes) != 0 {
		t.Fatalf("another organization saw the mailbox: %v, %v", boxes, err)
	}

	mine, err := repo.UnseenCountForMailboxes(ctx, f.org, []uuid.UUID{f.mailbox})
	if err != nil || mine != 1 {
		t.Fatalf("UnseenCountForMailboxes(own) = %d, %v", mine, err)
	}
	other, err := repo.UnseenCountForMailboxes(ctx, f.org, []uuid.UUID{uuid.New()})
	if err != nil || other != 0 {
		t.Fatalf("UnseenCountForMailboxes(other) = %d, %v", other, err)
	}

	ov, err := repo.OverviewForMailboxes(ctx, f.org, []uuid.UUID{uuid.New()})
	if err != nil {
		t.Fatalf("OverviewForMailboxes: %v", err)
	}
	if ov.Total != 0 || len(ov.Mailboxes) != 0 {
		t.Fatalf("a mailbox outside the set was counted: total %d, mailboxes %d", ov.Total, len(ov.Mailboxes))
	}
	ov, err = repo.OverviewForMailboxes(ctx, f.org, []uuid.UUID{f.mailbox})
	if err != nil || ov.Total != 1 || len(ov.Mailboxes) != 1 {
		t.Fatalf("OverviewForMailboxes(own) = %+v, %v", ov, err)
	}
}
