package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/models"
)

func TestLiveUniboxNotificationExcludesMachineMail(t *testing.T) {
	for _, tc := range []struct {
		kind               string
		automated          bool
		confidence         float64
		review             string
		wantReply, wantOOO bool
	}{
		{"notification", true, 0.95, "", false, false},
		{"notification", false, 0.95, "", false, false},
		{"bounce_hard", true, 0.95, "", false, false},
		{"bounce_soft", true, 0.95, "", false, false},
		{"auto_reply_ooo", true, 0.95, "", false, true},
		{"auto_reply_ticket", true, 0.95, "", false, true},
		{"human_reply", false, 0.95, "", true, true},
		{"notification", false, 0.3, "kind", true, true},
		{"notification", false, 0.95, "kind", true, true},
	} {
		t.Run(tc.kind+tc.review+map[bool]string{true: "-automated", false: "-inbox"}[tc.automated], func(t *testing.T) {
			handle := liveUniboxFolderDB(t)
			f := newUniboxFolderFixture(t, handle.Pool)
			ctx := context.Background()
			notifs := NewNotificationRepository(handle.Pool)
			thread := "thread-" + uuid.NewString()
			id := f.scopedMessage(t, NewUniboxRepository(handle), thread, "abuse@seznam.cz", models.FolderInbox, time.Now())
			due := time.Now().Add(-time.Minute)
			categories := []models.NotificationCategory{models.NotifInboundReply, models.NotifInboundOOO, models.NotifInboxActionRequired, models.NotifDomainAuth, models.NotifHealthBounce}
			for _, category := range categories {
				if _, err := notifs.Create(ctx, &models.Notification{UserID: f.user, OrganizationID: &f.org, UniboxEmailID: &id, Category: category, Title: "Alert", EmailState: "pending", EmailDueAt: &due}); err != nil {
					t.Fatal(err)
				}
			}
			if err := NewInboxTagRepository(handle.Pool).Save(ctx, &InboxTagResult{
				OrganizationID: f.org, EmailAccountID: f.mailbox, MessageID: "<" + id.String() + "@test.local>", ThreadID: thread,
				Kind: tc.kind, KindConfidence: tc.confidence, KindSource: "model", ReviewReason: tc.review, Priority: "whenever", Automated: tc.automated,
			}); err != nil {
				t.Fatal(err)
			}
			wantCount := 3
			if tc.wantReply {
				wantCount++
			}
			if tc.wantOOO {
				wantCount++
			}
			for _, unread := range []bool{false, true} {
				rows, err := notifs.List(ctx, f.user, 50, unread)
				if err != nil || len(rows) != wantCount {
					t.Fatalf("List(unread=%v)=%d (%v), want %d", unread, len(rows), err, wantCount)
				}
			}
			if count, err := notifs.CountUnread(ctx, f.user); err != nil || count != wantCount {
				t.Fatalf("CountUnread=%d (%v), want %d", count, err, wantCount)
			}
			for _, category := range categories {
				want := category != models.NotifInboundReply && category != models.NotifInboundOOO || category == models.NotifInboundReply && tc.wantReply || category == models.NotifInboundOOO && tc.wantOOO
				allowed, err := notifs.CanNotifyAboutMessage(ctx, id, category)
				if err != nil || allowed != want {
					t.Fatalf("CanNotifyAboutMessage(%s)=%v (%v), want %v", category, allowed, err, want)
				}
				_, err = notifs.Create(ctx, &models.Notification{UserID: f.user, OrganizationID: &f.org, UniboxEmailID: &id, Category: category, Title: "New alert", PreRead: true})
				if want && err != nil || !want && !errors.Is(err, ErrNotificationMessageAutomated) {
					t.Fatalf("Create(%s)=%v, allowed=%v", category, err, want)
				}
			}
			claimed, err := notifs.ClaimDueEmails(ctx)
			if err != nil {
				t.Fatal(err)
			}
			mine := 0
			for _, row := range claimed {
				if row.UserID == f.user {
					if row.UniboxEmailID == nil || *row.UniboxEmailID != id {
						t.Fatalf("claimed row lost its source message: %v", row.UniboxEmailID)
					}
					mine++
				}
			}
			if mine != wantCount {
				t.Fatalf("ClaimDueEmails claimed %d, want %d", mine, wantCount)
			}
		})
	}
}

func TestLiveUniboxNotificationLegacyReportIsMailboxScoped(t *testing.T) {
	handle := liveUniboxFolderDB(t)
	f := newUniboxFolderFixture(t, handle.Pool)
	ctx := context.Background()
	repo := NewUniboxRepository(handle)
	notifs := NewNotificationRepository(handle.Pool)
	thread := "legacy-report"
	id := f.scopedMessage(t, repo, thread, "abuse@seznam.cz", models.FolderInbox, time.Now())
	for _, mailbox := range []string{f.mailbox.String(), uuid.NewString(), "invalid-uuid"} {
		if _, err := notifs.Create(ctx, &models.Notification{UserID: f.user, OrganizationID: &f.org, Category: models.NotifInboundReply, Title: "Legacy reply", Metadata: map[string]any{"email_account_id": mailbox, "thread_id": thread}}); err != nil {
			t.Fatal(err)
		}
	}
	f.judge(t, handle.Pool, id, thread, true)
	if count, err := notifs.CountUnread(ctx, f.user); err != nil || count != 2 {
		t.Fatalf("legacy count=%d (%v), want 2 unmatched mailbox rows", count, err)
	}
	human := f.scopedMessage(t, repo, thread, "person@example.test", models.FolderInbox, time.Now())
	f.judge(t, handle.Pool, human, thread, false)
	if count, err := notifs.CountUnread(ctx, f.user); err != nil || count != 3 {
		t.Fatalf("mixed legacy count=%d (%v), want all 3", count, err)
	}
}

// The unread badge and a reply notification both point at the Inbox. Each has
// to agree with what the Inbox lists, or a click lands on nothing (#659):
//
//	WARMBLY_TEST_DB=postgres://warmbly:warmbly@localhost:15432/<db>?sslmode=disable \
//	  go test ./internal/repository/ -run LiveUniboxNotification -v

// The badge links to Inbox, so it counts what Inbox shows as unread and
// nothing that view leaves out.
func TestLiveUniboxNotificationBadgeCountsOnlyWhatInboxLists(t *testing.T) {
	handle := liveUniboxFolderDB(t)
	f := newUniboxFolderFixture(t, handle.Pool)
	repo := NewUniboxRepository(handle)
	ctx := context.Background()
	now := time.Now().UTC()

	f.scopedMessage(t, repo, "thread-inbox", "them@example.com", models.FolderInbox, now)
	f.scopedMessage(t, repo, "thread-sent", "them@example.com", models.FolderSent, now)
	f.scopedMessage(t, repo, "thread-draft", "them@example.com", models.FolderDrafts, now)
	f.scopedMessage(t, repo, "thread-snoozed", "them@example.com", models.FolderInbox, now)
	if _, err := repo.UpsertSnoozes(ctx, f.org, f.user, []string{"thread-snoozed"}, now.Add(24*time.Hour)); err != nil {
		t.Fatalf("UpsertSnoozes: %v", err)
	}

	inbox, unseen := models.FolderInbox, true
	listed, err := repo.Search(ctx, f.org, &models.MailSearchParams{Folder: &inbox, Unseen: &unseen, PageSize: 50})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	count, err := repo.GetUnseenCount(ctx, f.org, nil)
	if err != nil {
		t.Fatalf("GetUnseenCount: %v", err)
	}
	if count != 1 || len(listed.Data) != 1 {
		t.Fatalf("badge = %d, unread Inbox rows = %v; want both 1 (only thread-inbox)", count, threadIDs(listed))
	}

	mine, err := repo.GetUnseenCount(ctx, f.org, &f.mailbox)
	if err != nil {
		t.Fatalf("GetUnseenCount for the mailbox: %v", err)
	}
	if mine != 1 {
		t.Fatalf("mailbox badge = %d, want 1", mine)
	}
}

// Reading the message reads its notification, on the in-app path and on the
// sync path alike, and its pending digest email is cancelled with it.
func TestLiveUniboxNotificationIsReadWithTheMessage(t *testing.T) {
	handle := liveUniboxFolderDB(t)
	f := newUniboxFolderFixture(t, handle.Pool)
	repo := NewUniboxRepository(handle)
	notifs := NewNotificationRepository(handle.Pool)
	ctx := context.Background()
	now := time.Now().UTC()

	inApp := f.scopedMessage(t, repo, "thread-read-here", "them@example.com", models.FolderInbox, now)
	synced := f.scopedMessage(t, repo, "thread-read-there", "them@example.com", models.FolderInbox, now)
	other := f.scopedMessage(t, repo, "thread-unread", "them@example.com", models.FolderInbox, now)

	due := now.Add(time.Hour)
	create := func(id uuid.UUID) uuid.UUID {
		n, err := notifs.Create(ctx, &models.Notification{
			UserID: f.user, OrganizationID: &f.org, Category: models.NotifInboundReply,
			Title: "New reply", UniboxEmailID: &id, EmailState: "pending", EmailDueAt: &due,
		})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		return n.ID
	}
	nInApp, nSynced, nOther := create(inApp), create(synced), create(other)

	if _, err := repo.MarkSeenByThreads(ctx, f.org, []string{"thread-read-here"}, true); err != nil {
		t.Fatalf("MarkSeenByThreads: %v", err)
	}
	seen := true
	if err := repo.UpdateEntry(ctx, f.user, f.mailbox, synced, &UpdateUniboxEntry{Seen: &seen}); err != nil {
		t.Fatalf("UpdateEntry: %v", err)
	}

	state := func(id uuid.UUID) (bool, string) {
		var read bool
		var email string
		if err := handle.Pool.QueryRow(ctx,
			`SELECT read_at IS NOT NULL, email_state FROM notifications WHERE id = $1`, id).Scan(&read, &email); err != nil {
			t.Fatalf("read notification: %v", err)
		}
		return read, email
	}
	for name, id := range map[string]uuid.UUID{"in-app read": nInApp, "provider read": nSynced} {
		if read, email := state(id); !read || email != "skipped" {
			t.Errorf("%s: read=%v email=%q, want read with its email skipped", name, read, email)
		}
	}
	if read, email := state(nOther); read || email != "pending" {
		t.Errorf("untouched message: read=%v email=%q, want its notification left alone", read, email)
	}
}

// A message that leaves the unibox (deleted, or found to be warmup by the
// sweep) takes its notification with it.
func TestLiveUniboxNotificationLeavesWithTheMessage(t *testing.T) {
	handle := liveUniboxFolderDB(t)
	f := newUniboxFolderFixture(t, handle.Pool)
	repo := NewUniboxRepository(handle)
	notifs := NewNotificationRepository(handle.Pool)
	ctx := context.Background()

	id := f.scopedMessage(t, repo, "thread-leaves", "them@example.com", models.FolderInbox, time.Now().UTC())
	n, err := notifs.Create(ctx, &models.Notification{
		UserID: f.user, OrganizationID: &f.org, Category: models.NotifInboundReply,
		Title: "New reply", UniboxEmailID: &id,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Delete(ctx, f.user, id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	var left int
	if err := handle.Pool.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE id = $1`, n.ID).Scan(&left); err != nil {
		t.Fatalf("count: %v", err)
	}
	if left != 0 {
		t.Fatal("the notification outlived the message it was about")
	}

	// A message already gone cannot be announced.
	if _, err := notifs.Create(ctx, &models.Notification{
		UserID: f.user, OrganizationID: &f.org, Category: models.NotifInboundReply,
		Title: "New reply", UniboxEmailID: &id,
	}); !errors.Is(err, ErrNotificationMessageGone) {
		t.Fatalf("error = %v, want ErrNotificationMessageGone", err)
	}
}

// A message read before its notification is written (read in the mail client
// before the sync, or in a race with the reader) is announced as read, and
// an email-only notification is cancelled when its message is read.
func TestLiveUniboxNotificationRespectsTheMessagesReadState(t *testing.T) {
	handle := liveUniboxFolderDB(t)
	f := newUniboxFolderFixture(t, handle.Pool)
	repo := NewUniboxRepository(handle)
	notifs := NewNotificationRepository(handle.Pool)
	ctx := context.Background()
	now := time.Now().UTC()
	due := now.Add(time.Hour)

	state := func(id uuid.UUID) (bool, string) {
		var read bool
		var email string
		if err := handle.Pool.QueryRow(ctx,
			`SELECT read_at IS NOT NULL, email_state FROM notifications WHERE id = $1`, id).Scan(&read, &email); err != nil {
			t.Fatalf("read notification: %v", err)
		}
		return read, email
	}

	already := f.scopedMessage(t, repo, "thread-already-read", "them@example.com", models.FolderInbox, now)
	if _, err := repo.MarkSeenByThreads(ctx, f.org, []string{"thread-already-read"}, true); err != nil {
		t.Fatalf("MarkSeenByThreads: %v", err)
	}
	n, err := notifs.Create(ctx, &models.Notification{
		UserID: f.user, OrganizationID: &f.org, Category: models.NotifInboundReply,
		Title: "New reply", UniboxEmailID: &already, EmailState: "pending", EmailDueAt: &due,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !n.MessageSeen {
		t.Error("Create did not report the message as already read")
	}
	if read, email := state(n.ID); !read || email != "skipped" {
		t.Errorf("already-read message: read=%v email=%q, want read with no email", read, email)
	}

	emailOnly := f.scopedMessage(t, repo, "thread-email-only", "them@example.com", models.FolderInbox, now)
	n, err = notifs.Create(ctx, &models.Notification{
		UserID: f.user, OrganizationID: &f.org, Category: models.NotifInboundReply,
		Title: "New reply", UniboxEmailID: &emailOnly, EmailState: "pending", EmailDueAt: &due, PreRead: true,
	})
	if err != nil {
		t.Fatalf("Create email-only: %v", err)
	}
	if n.MessageSeen {
		t.Error("an unread message was reported as read")
	}
	if _, err := repo.MarkSeenByThreads(ctx, f.org, []string{"thread-email-only"}, true); err != nil {
		t.Fatalf("MarkSeenByThreads: %v", err)
	}
	if read, email := state(n.ID); !read || email != "skipped" {
		t.Errorf("email-only notification: read=%v email=%q, want its email cancelled by the read", read, email)
	}
}
