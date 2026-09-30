package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/models"
)

// What a view shows after a conversation is filed, and who "we spoke last"
// resolves to. Both are decided entirely in SQL, which is why they run against
// a real schema:
//
//	WARMBLY_TEST_DB=postgres://warmbly:warmbly@localhost:15432/warmbly_dev?sslmode=disable \
//	  go test ./internal/repository/ -run LiveUniboxScope -v
//
// Archive used to leave a conversation in every view but Inbox, so pressing it
// from Awaiting reply removed the row and the next read put it straight back.
// Awaiting reply compared a mailbox address against the whole From header,
// which is "Name <addr>" from every provider, so it matched almost nothing.

// message writes one message into the fixture's mailbox with the from header
// and folder the case needs. threadID groups them into a conversation.
func (f *uniboxFolderFixture) scopedMessage(t *testing.T, repo UniboxRepository, threadID, from, folder string, at time.Time) uuid.UUID {
	t.Helper()
	id := uuid.New()
	err := repo.CreateEntry(context.Background(), f.user, &models.EmailMessageStoreData{
		ID: id, EmailID: f.mailbox, Folder: folder,
		ThreadID: threadID, MessageID: "<" + id.String() + "@test.local>",
		FromAddr: []string{from},
		ToAddr:   []string{"them@example.com"},
		Subject:  "Scope", Snippet: "Scope",
		InternalDate: at, SentDate: at, CreatedAt: at, UpdatedAt: at,
		Seen: false,
	})
	if err != nil {
		t.Fatalf("CreateEntry: %v", err)
	}
	return id
}

func (f *uniboxFolderFixture) mailboxAddress(t *testing.T, repo UniboxRepository) string {
	t.Helper()
	ov, err := repo.Overview(context.Background(), f.org)
	if err != nil {
		t.Fatalf("Overview: %v", err)
	}
	if len(ov.Mailboxes) != 1 {
		t.Fatalf("fixture has %d mailboxes, want 1", len(ov.Mailboxes))
	}
	return ov.Mailboxes[0].Email
}

func threadIDs(res *models.MailSearchResult) []string {
	out := make([]string, 0, len(res.Data))
	for _, row := range res.Data {
		out = append(out, row.ThreadID)
	}
	return out
}

func listsThread(res *models.MailSearchResult, want string) bool {
	for _, id := range threadIDs(res) {
		if id == want {
			return true
		}
	}
	return false
}

// A mailbox sends as "Name <addr>". Comparing that against the address whole
// is what made Awaiting reply look empty on a workspace that had been sending
// all week.
func TestLiveUniboxScopeAwaitingReplyReadsTheAddressOutOfTheHeader(t *testing.T) {
	handle := liveUniboxFolderDB(t)
	f := newUniboxFolderFixture(t, handle.Pool)
	repo := NewUniboxRepository(handle)
	ctx := context.Background()
	ours := f.mailboxAddress(t, repo)
	now := time.Now().UTC()

	f.scopedMessage(t, repo, "thread-wrapped", "Alex at Acme <"+ours+">", models.FolderSent, now)
	f.scopedMessage(t, repo, "thread-theirs", "them@example.com", models.FolderInbox, now)

	awaiting := true
	res, err := repo.Search(ctx, f.org, &models.MailSearchParams{AwaitingReply: &awaiting, PageSize: 50})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if !listsThread(res, "thread-wrapped") {
		t.Fatalf("awaiting reply = %v, want the thread we spoke last on", threadIDs(res))
	}
	if listsThread(res, "thread-theirs") {
		t.Fatalf("awaiting reply = %v, want no thread the other side spoke last on", threadIDs(res))
	}

	ov, err := repo.Overview(ctx, f.org)
	if err != nil {
		t.Fatalf("Overview: %v", err)
	}
	if ov.AwaitingReply != 1 {
		t.Fatalf("overview awaiting_reply = %d, want 1 (the rail has to agree with the rows)", ov.AwaitingReply)
	}
}

// An address that merely contains ours is not ours.
func TestLiveUniboxScopeAwaitingReplyIsNotASubstringMatch(t *testing.T) {
	handle := liveUniboxFolderDB(t)
	f := newUniboxFolderFixture(t, handle.Pool)
	repo := NewUniboxRepository(handle)
	ctx := context.Background()
	ours := f.mailboxAddress(t, repo)

	f.scopedMessage(t, repo, "thread-lookalike", "Impostor <x"+ours+">", models.FolderInbox, time.Now().UTC())

	awaiting := true
	res, err := repo.Search(ctx, f.org, &models.MailSearchParams{AwaitingReply: &awaiting, PageSize: 50})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if listsThread(res, "thread-lookalike") {
		t.Fatalf("awaiting reply = %v, want an address that only contains ours left out", threadIDs(res))
	}
}

// Filing takes the conversation out of every view a workspace works from, and
// leaves it in All mail and in Archive. Anything less and the row comes back on
// the next read, which is what "Archive does nothing" looked like.
func TestLiveUniboxScopeArchiveLeavesTheWorkingViews(t *testing.T) {
	handle := liveUniboxFolderDB(t)
	f := newUniboxFolderFixture(t, handle.Pool)
	repo := NewUniboxRepository(handle)
	ctx := context.Background()
	ours := f.mailboxAddress(t, repo)
	now := time.Now().UTC()

	f.scopedMessage(t, repo, "thread-filed", "Alex at Acme <"+ours+">", models.FolderSent, now)

	awaiting, unseen, yes := true, true, true
	before, err := repo.Search(ctx, f.org, &models.MailSearchParams{AwaitingReply: &awaiting, PageSize: 50})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if !listsThread(before, "thread-filed") {
		t.Fatal("the conversation is not in Awaiting reply to begin with")
	}

	if _, err := repo.MoveThreadsToFolder(ctx, f.org, []string{"thread-filed"}, models.FolderArchive); err != nil {
		t.Fatalf("MoveThreadsToFolder: %v", err)
	}

	for _, tc := range []struct {
		view   string
		params *models.MailSearchParams
	}{
		{"unscoped", &models.MailSearchParams{PageSize: 50}},
		{"awaiting reply", &models.MailSearchParams{AwaitingReply: &awaiting, PageSize: 50}},
		{"unread", &models.MailSearchParams{Unseen: &unseen, PageSize: 50}},
	} {
		res, err := repo.Search(ctx, f.org, tc.params)
		if err != nil {
			t.Fatalf("Search %s: %v", tc.view, err)
		}
		if listsThread(res, "thread-filed") {
			t.Errorf("a filed conversation is still in %s", tc.view)
		}
	}

	allMail, err := repo.Search(ctx, f.org, &models.MailSearchParams{IncludeArchived: &yes, PageSize: 50})
	if err != nil {
		t.Fatalf("Search all mail: %v", err)
	}
	if !listsThread(allMail, "thread-filed") {
		t.Error("a filed conversation has left All mail too; Archive is where it went, not nowhere")
	}

	archive := models.FolderArchive
	filed, err := repo.Search(ctx, f.org, &models.MailSearchParams{Folder: &archive, PageSize: 50})
	if err != nil {
		t.Fatalf("Search archive: %v", err)
	}
	if !listsThread(filed, "thread-filed") {
		t.Error("a filed conversation is not in the Archive folder")
	}

	ov, err := repo.Overview(ctx, f.org)
	if err != nil {
		t.Fatalf("Overview: %v", err)
	}
	if ov.AwaitingReply != 0 || ov.Unread != 0 {
		t.Errorf("overview awaiting=%d unread=%d, want both 0 once the only conversation is filed", ov.AwaitingReply, ov.Unread)
	}
	if ov.Total != 1 {
		t.Errorf("overview total = %d, want 1: All mail still holds it", ov.Total)
	}
	unseenCount, err := repo.GetUnseenCount(ctx, f.org, nil)
	if err != nil {
		t.Fatalf("GetUnseenCount: %v", err)
	}
	if unseenCount != 0 {
		t.Errorf("unread badge = %d, want 0", unseenCount)
	}
}

// Filing by conversation has to take every message in it. Leaving one behind
// puts the whole row back, because a thread is shown by any message of it that
// is still in a working folder.
func TestLiveUniboxScopeArchiveTakesTheWholeConversation(t *testing.T) {
	handle := liveUniboxFolderDB(t)
	f := newUniboxFolderFixture(t, handle.Pool)
	repo := NewUniboxRepository(handle)
	ctx := context.Background()
	now := time.Now().UTC()

	f.scopedMessage(t, repo, "thread-pair", "them@example.com", models.FolderInbox, now.Add(-time.Hour))
	f.scopedMessage(t, repo, "thread-pair", "them@example.com", models.FolderInbox, now)

	if _, err := repo.MoveThreadsToFolder(ctx, f.org, []string{"thread-pair"}, models.FolderArchive); err != nil {
		t.Fatalf("MoveThreadsToFolder: %v", err)
	}

	res, err := repo.Search(ctx, f.org, &models.MailSearchParams{PageSize: 50})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if listsThread(res, "thread-pair") {
		t.Fatal("filing the conversation left part of it behind, so the row is still listed")
	}
}

// Another organization's conversations are not this one's to file.
func TestLiveUniboxScopeMoveThreadsIsOrgScoped(t *testing.T) {
	handle := liveUniboxFolderDB(t)
	mine := newUniboxFolderFixture(t, handle.Pool)
	theirs := newUniboxFolderFixture(t, handle.Pool)
	repo := NewUniboxRepository(handle)
	ctx := context.Background()

	id := theirs.scopedMessage(t, repo, "thread-theirs-only", "them@example.com", models.FolderInbox, time.Now().UTC())
	if _, err := repo.MoveThreadsToFolder(ctx, mine.org, []string{"thread-theirs-only"}, models.FolderArchive); err != nil {
		t.Fatalf("MoveThreadsToFolder: %v", err)
	}

	got, err := repo.GetByID(ctx, theirs.user, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Folder != models.FolderInbox {
		t.Fatalf("folder = %q, want another org's conversation left at %q", got.Folder, models.FolderInbox)
	}
}

// Marking a conversation read from a list row addresses it by thread, because
// the row knows the conversation and not the ids inside it.
func TestLiveUniboxScopeMarkSeenByThreadsTakesTheWholeConversation(t *testing.T) {
	handle := liveUniboxFolderDB(t)
	f := newUniboxFolderFixture(t, handle.Pool)
	repo := NewUniboxRepository(handle)
	ctx := context.Background()
	now := time.Now().UTC()

	f.scopedMessage(t, repo, "thread-unread", "them@example.com", models.FolderInbox, now.Add(-time.Hour))
	f.scopedMessage(t, repo, "thread-unread", "them@example.com", models.FolderInbox, now)

	changed, err := repo.MarkSeenByThreads(ctx, f.org, []string{"thread-unread"}, true)
	if err != nil {
		t.Fatalf("MarkSeenByThreads: %v", err)
	}
	if len(changed) != 2 {
		t.Fatalf("changed %d messages, want both in the conversation", len(changed))
	}

	count, err := repo.GetUnseenCount(ctx, f.org, nil)
	if err != nil {
		t.Fatalf("GetUnseenCount: %v", err)
	}
	if count != 0 {
		t.Fatalf("unread = %d, want 0", count)
	}

	// Already read costs nothing at the provider: only real changes relay.
	again, err := repo.MarkSeenByThreads(ctx, f.org, []string{"thread-unread"}, true)
	if err != nil {
		t.Fatalf("MarkSeenByThreads again: %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("changed %d messages on a no-op mark, want 0", len(again))
	}
}

// The selection bar snoozes a whole screenful in one call, so the repository
// takes the set. Upsert semantics hold per row: re-snoozing a conversation
// already in the set moves its time rather than failing the batch.
func TestLiveUniboxScopeSnoozeTakesASet(t *testing.T) {
	handle := liveUniboxFolderDB(t)
	f := newUniboxFolderFixture(t, handle.Pool)
	repo := NewUniboxRepository(handle)
	ctx := context.Background()
	now := time.Now().UTC()

	f.scopedMessage(t, repo, "thread-a", "them@example.com", models.FolderInbox, now)
	f.scopedMessage(t, repo, "thread-b", "them@example.com", models.FolderInbox, now)
	t.Cleanup(func() {
		_ = repo.DeleteSnoozes(context.Background(), f.user, []string{"thread-a", "thread-b"})
	})

	rows, err := repo.UpsertSnoozes(ctx, f.user, []string{"thread-a", "thread-b"}, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("UpsertSnoozes: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("snoozed %d conversations, want 2", len(rows))
	}

	res, err := repo.Search(ctx, f.org, &models.MailSearchParams{PageSize: 50})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if listsThread(res, "thread-a") || listsThread(res, "thread-b") {
		t.Fatalf("a snoozed conversation is still listed: %v", threadIDs(res))
	}

	// A second pass over the same set moves the time instead of erroring.
	if _, err := repo.UpsertSnoozes(ctx, f.user, []string{"thread-a", "thread-b"}, now.Add(2*time.Hour)); err != nil {
		t.Fatalf("UpsertSnoozes again: %v", err)
	}

	if err := repo.DeleteSnoozes(ctx, f.user, []string{"thread-a", "thread-b"}); err != nil {
		t.Fatalf("DeleteSnoozes: %v", err)
	}
	back, err := repo.Search(ctx, f.org, &models.MailSearchParams{PageSize: 50})
	if err != nil {
		t.Fatalf("Search after un-snooze: %v", err)
	}
	if !listsThread(back, "thread-a") || !listsThread(back, "thread-b") {
		t.Fatalf("un-snoozing did not bring the conversations back: %v", threadIDs(back))
	}
}
