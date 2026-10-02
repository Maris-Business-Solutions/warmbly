package repository

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/warmbly/warmbly/internal/models"
)

// Mark as unread follows Gmail: the newest message received in the
// conversation turns unread, and a sent copy never does.
//
//	WARMBLY_TEST_DB=postgres://warmbly:warmbly@localhost:15432/warmbly_dev?sslmode=disable \
//	  go test ./internal/repository/ -run LiveUniboxMarkUnread -v

func (f *uniboxFolderFixture) threadMessage(t *testing.T, repo UniboxRepository, thread, folder string, age time.Duration) uuid.UUID {
	t.Helper()
	id := uuid.New()
	at := time.Now().UTC().Add(-age)
	err := repo.CreateEntry(context.Background(), f.user, &models.EmailMessageStoreData{
		ID: id, EmailID: f.mailbox, Folder: folder,
		ThreadID: thread, MessageID: "<" + id.String() + "@test.local>",
		FromAddr: []string{"them@example.com"},
		ToAddr:   []string{"me@test.local"},
		Subject:  "Unread", Snippet: "Unread",
		InternalDate: at, SentDate: at, CreatedAt: at, UpdatedAt: at,
		Seen: true,
	})
	if err != nil {
		t.Fatalf("CreateEntry: %v", err)
	}
	return id
}

// Read straight from the table: GetByID marks what it returns as read.
func unreadIDs(t *testing.T, pool *pgxpool.Pool, ids ...uuid.UUID) []uuid.UUID {
	t.Helper()
	var unread []uuid.UUID
	for _, id := range ids {
		var seen bool
		if err := pool.QueryRow(context.Background(), `SELECT seen FROM unibox_emails WHERE id = $1`, id).Scan(&seen); err != nil {
			t.Fatalf("read seen: %v", err)
		}
		if !seen {
			unread = append(unread, id)
		}
	}
	return unread
}

func TestLiveUniboxMarkUnreadByThreadTouchesOnlyTheNewestReceived(t *testing.T) {
	handle := liveUniboxFolderDB(t)
	f := newUniboxFolderFixture(t, handle.Pool)
	repo := NewUniboxRepository(handle)
	ctx := context.Background()

	thread := "thread-" + uuid.NewString()
	first := f.threadMessage(t, repo, thread, models.FolderArchive, 3*time.Hour)
	reply := f.threadMessage(t, repo, thread, models.FolderSent, 2*time.Hour)
	answer := f.threadMessage(t, repo, thread, models.FolderInbox, time.Hour)
	// Ours and newer than the answer: still never the one marked.
	followUp := f.threadMessage(t, repo, thread, models.FolderSent, time.Minute)

	changed, err := repo.MarkSeenByThreads(ctx, f.org, []string{thread}, false)
	if err != nil {
		t.Fatalf("MarkSeenByThreads: %v", err)
	}
	if !slices.Equal(changed, []uuid.UUID{answer}) {
		t.Fatalf("changed = %v, want only the newest received %v", changed, answer)
	}
	if got := unreadIDs(t, handle.Pool, first, reply, answer, followUp); !slices.Equal(got, []uuid.UUID{answer}) {
		t.Fatalf("unread = %v, want only %v", got, answer)
	}

	// Read still covers the whole conversation.
	if _, err := repo.MarkSeenByThreads(ctx, f.org, []string{thread}, true); err != nil {
		t.Fatalf("MarkSeenByThreads: %v", err)
	}
	if got := unreadIDs(t, handle.Pool, first, reply, answer, followUp); len(got) != 0 {
		t.Fatalf("unread after read = %v, want none", got)
	}
}

// A newer copy in Trash would leave the Inbox row reading as read.
func TestLiveUniboxMarkUnreadByThreadPrefersMailOutsideTrash(t *testing.T) {
	handle := liveUniboxFolderDB(t)
	f := newUniboxFolderFixture(t, handle.Pool)
	repo := NewUniboxRepository(handle)

	thread := "thread-" + uuid.NewString()
	inbox := f.threadMessage(t, repo, thread, models.FolderInbox, 2*time.Hour)
	trashed := f.threadMessage(t, repo, thread, models.FolderTrash, time.Hour)

	changed, err := repo.MarkSeenByThreads(context.Background(), f.org, []string{thread}, false)
	if err != nil {
		t.Fatalf("MarkSeenByThreads: %v", err)
	}
	if !slices.Equal(changed, []uuid.UUID{inbox}) {
		t.Fatalf("changed = %v, want the inbox copy %v (not %v)", changed, inbox, trashed)
	}
}

func TestLiveUniboxMarkUnreadNeverReachesASentCopy(t *testing.T) {
	handle := liveUniboxFolderDB(t)
	f := newUniboxFolderFixture(t, handle.Pool)
	repo := NewUniboxRepository(handle)
	ctx := context.Background()

	sentOnly := "thread-" + uuid.NewString()
	sent := f.threadMessage(t, repo, sentOnly, models.FolderSent, time.Hour)
	draft := f.threadMessage(t, repo, "thread-"+uuid.NewString(), models.FolderDrafts, time.Hour)
	inbox := f.threadMessage(t, repo, "thread-"+uuid.NewString(), models.FolderInbox, time.Hour)

	if changed, err := repo.MarkSeenByThreads(ctx, f.org, []string{sentOnly}, false); err != nil || len(changed) != 0 {
		t.Fatalf("by thread: changed = %v, err = %v; want nothing", changed, err)
	}
	changed, err := repo.MarkSeenBulk(ctx, f.org, []uuid.UUID{sent, draft, inbox}, false)
	if err != nil {
		t.Fatalf("MarkSeenBulk: %v", err)
	}
	if !slices.Equal(changed, []uuid.UUID{inbox}) {
		t.Fatalf("by id: changed = %v, want only %v", changed, inbox)
	}
	if changed, err := repo.MarkSeenByFolder(ctx, f.org, models.FolderSent, false); err != nil || len(changed) != 0 {
		t.Fatalf("by folder: changed = %v, err = %v; want nothing", changed, err)
	}
	if got := unreadIDs(t, handle.Pool, sent, draft); len(got) != 0 {
		t.Fatalf("unread = %v, want no sent or draft copy", got)
	}
}
