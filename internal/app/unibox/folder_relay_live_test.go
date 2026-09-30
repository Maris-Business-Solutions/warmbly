package unibox

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

// Live cover for the filing relay's two statements: the filing reports which
// rows it moved out of another folder, and the targets carry the provider's
// placement while leaving out mailboxes that opted out and warmup receipts.
func TestLiveFolderRelayReportsMovesAndResolvesTargets(t *testing.T) {
	f := newSeenLiveFixture(t)
	ctx := context.Background()

	filed, err := f.repo.MoveToFolderBulk(ctx, f.org, []uuid.UUID{f.unread, f.read}, models.FolderArchive)
	if err != nil {
		t.Fatalf("file: %v", err)
	}
	if len(filed) != 2 || !filed[0].Moved || !filed[1].Moved {
		t.Fatalf("both rows left the inbox: %+v", filed)
	}
	// Filing again matches both and moves neither.
	filed, err = f.repo.MoveToFolderBulk(ctx, f.org, []uuid.UUID{f.unread, f.read}, models.FolderArchive)
	if err != nil {
		t.Fatalf("file again: %v", err)
	}
	if len(filed) != 2 || filed[0].Moved || filed[1].Moved {
		t.Fatalf("a repeated filing reported a move: %+v", filed)
	}

	targets, err := f.repo.FolderRelayTargets(ctx, f.org, []uuid.UUID{f.unread, f.read})
	if err != nil {
		t.Fatalf("targets: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("expected both messages, got %d", len(targets))
	}
	for _, tg := range targets {
		if tg.WorkerID != f.worker || tg.Provider != "gmail" || tg.Folder != models.FolderArchive {
			t.Errorf("target %+v", tg)
		}
		if tg.Ref.ProviderFolder != "" || tg.Ref.ProviderID == "" || tg.Ref.RFCMessageID == "" {
			t.Errorf("provider handles %+v", tg.Ref)
		}
	}

	// A warmup receipt is never moved: the ladder would read it as tampering.
	if _, err := f.pool.Exec(ctx, `INSERT INTO warmup_received (email_account_id, internal_id, message_id, sender_account_id)
	      VALUES ($1, $2, '<read@test>', $1)`, f.box, f.read); err != nil {
		t.Fatalf("receipt: %v", err)
	}
	t.Cleanup(func() {
		_, _ = f.pool.Exec(context.Background(), `DELETE FROM warmup_received WHERE email_account_id = $1`, f.box)
	})
	targets, err = f.repo.FolderRelayTargets(ctx, f.org, []uuid.UUID{f.unread, f.read})
	if err != nil {
		t.Fatalf("targets: %v", err)
	}
	if len(targets) != 1 || targets[0].Ref.ID != f.unread {
		t.Fatalf("the warmup receipt was offered for a move: %+v", targets)
	}

	// A mailbox that turned the relay off offers nothing.
	if _, err := f.pool.Exec(ctx, `UPDATE email_accounts SET relay_folder_moves = false WHERE id = $1`, f.box); err != nil {
		t.Fatalf("opt out: %v", err)
	}
	targets, err = f.repo.FolderRelayTargets(ctx, f.org, []uuid.UUID{f.unread})
	if err != nil {
		t.Fatalf("targets: %v", err)
	}
	if len(targets) != 0 {
		t.Fatalf("an opted-out mailbox was offered for a move: %+v", targets)
	}

	// Another workspace resolves nothing.
	targets, err = f.repo.FolderRelayTargets(ctx, uuid.New(), []uuid.UUID{f.unread})
	if err != nil || len(targets) != 0 {
		t.Fatalf("cross-workspace targets: %+v, %v", targets, err)
	}
}

// The answer to a relayed move writes the provider's side and nothing else.
func TestLiveRelayedAnswerMovesOnlyTheProviderSide(t *testing.T) {
	f := newSeenLiveFixture(t)
	ctx := context.Background()

	pf, uid, mailbox, path, gid := models.FolderArchive, uint32(90), uint32(7), "Archive", "moved-id"
	if err := f.repo.UpdateEntry(ctx, f.user, f.box, f.unread, &repository.UpdateUniboxEntry{
		ProviderFolder: &pf, UID: &uid, Mailbox: &mailbox, FolderPath: &path, ProviderID: &gid,
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err := f.repo.GetByID(ctx, f.user, f.unread)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.Folder != models.FolderInbox || got.ProviderFolder != pf || got.UID != uid || got.FolderPath != path || got.GmailID != gid {
		t.Fatalf("row after the answer: folder=%q provider=%q uid=%d path=%q gmail_id=%q",
			got.Folder, got.ProviderFolder, got.UID, got.FolderPath, got.GmailID)
	}
}
