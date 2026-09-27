package tasks

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/app/advanced"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

// A campaign rotating across several mailboxes: each lead's reply lands in
// the mailbox that wrote to it, and has to count for that lead whichever
// mailbox rotation picked. Real ticks pick the mailbox, the worker's
// confirmation is replayed as the consumer records it, and the reply goes
// through the real reply processor off a row stored the way the IMAP sync
// stores one.
//
//	WARMBLY_TEST_DB=postgres://warmbly:warmbly@localhost:15432/warmbly_dev?sslmode=disable \
//	  go test ./internal/tasks/ -run LiveMultiSender -v

type rotatedSend struct {
	task, mailbox, lead uuid.UUID
	mailboxEmail        string
	leadEmail           string
	messageID           string
}

// sendFromEachMailbox runs one tick per mailbox and returns what each one
// sent, failing unless rotation really spread the leads across the pool.
func (f *rotationFixture) sendFromEachMailbox(t *testing.T) []rotatedSend {
	t.Helper()
	ctx := context.Background()
	seed := f.mailboxes[0]
	var sends []rotatedSend
	used := map[uuid.UUID]bool{}
	for i := range f.mailboxes {
		taskID := f.tick(t, seed)
		mb, ok := f.sender.sentFrom(taskID)
		if !ok {
			t.Fatalf("tick %d dispatched nothing", i+1)
		}
		msg, _ := f.sender.message(taskID)
		if msg.MessageID == "" {
			t.Fatalf("tick %d sent without a Message-ID", i+1)
		}
		// The worker confirms the send and the consumer records the id the
		// recipient will answer to (HandleEmailSent).
		if err := f.svc.taskRepo.UpdateTaskMessageID(ctx, taskID, msg.MessageID); err != nil {
			t.Fatalf("record message id: %v", err)
		}
		s := rotatedSend{task: taskID, mailbox: mb, messageID: msg.MessageID}
		if err := f.pool.QueryRow(ctx, `SELECT contact_id FROM campaign_tasks WHERE task_id = $1`, taskID).Scan(&s.lead); err != nil {
			t.Fatalf("read the lead the task went to: %v", err)
		}
		if err := f.pool.QueryRow(ctx, `SELECT email FROM contacts WHERE id = $1`, s.lead).Scan(&s.leadEmail); err != nil {
			t.Fatalf("read lead email: %v", err)
		}
		if err := f.pool.QueryRow(ctx, `SELECT email FROM email_accounts WHERE id = $1`, mb).Scan(&s.mailboxEmail); err != nil {
			t.Fatalf("read mailbox email: %v", err)
		}
		used[mb] = true
		sends = append(sends, s)
	}
	if len(used) != len(f.mailboxes) {
		t.Fatalf("precondition: rotation used %d of %d mailboxes, so this would not exercise a multi-sender campaign", len(used), len(f.mailboxes))
	}
	return sends
}

// replyProcessor is the real reply pipeline over the fixture's repositories.
func (f *rotationFixture) replyProcessor() advanced.Service {
	return advanced.NewService(
		repository.NewAdvancedOutreachRepository(f.pool),
		f.svc.campaignRepo,
		f.svc.emailRepo,
		f.svc.taskRepo,
		f.svc.contactRepo,
		f.svc.campaignProgressRepo,
		repository.NewCRMRepository(f.pool),
		repository.NewGroupRepostory(f.handle, models.Categories),
		repository.NewUniboxRepository(f.handle),
		noopTaskScheduler{},
		nil,
	)
}

// storeReply files a reply to one send into a mailbox's inbox exactly as the
// IMAP sync stores it: addresses in the "Name (addr)" form, the sent
// Message-ID in In-Reply-To.
func (f *rotationFixture) storeReply(t *testing.T, into uuid.UUID, s rotatedSend) *models.EmailMessageStoreData {
	t.Helper()
	now := time.Now().UTC()
	msg := &models.EmailMessageStoreData{
		ID:           uuid.New(),
		EmailID:      into,
		Folder:       models.FolderInbox,
		FolderPath:   "INBOX",
		MessageID:    "<reply-" + uuid.New().String() + "@test.local>",
		InReplyTo:    []string{s.messageID},
		FromAddr:     []string{"Live Contact (" + s.leadEmail + ")"},
		ToAddr:       []string{"Live (" + s.mailboxEmail + ")"},
		Subject:      "Re: Hi",
		Snippet:      "Sounds good, let's set up a call next week.",
		InternalDate: now,
		SentDate:     now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := repository.NewUniboxRepository(f.handle).CreateEntry(context.Background(), f.user, msg); err != nil {
		t.Fatalf("store reply: %v", err)
	}
	return msg
}

func (f *rotationFixture) repliedAt(t *testing.T, lead uuid.UUID) *time.Time {
	t.Helper()
	var at *time.Time
	if err := f.pool.QueryRow(context.Background(),
		`SELECT replied_at FROM campaign_contact_progress WHERE campaign_id = $1 AND contact_id = $2 AND sequence_id = $3`,
		f.campaign, lead, f.step).Scan(&at); err != nil {
		t.Fatalf("read progress for lead %s: %v", lead, err)
	}
	return at
}

// TestLiveMultiSenderRepliesCountForEveryMailbox: a reply to each rotated
// mailbox stamps its own lead, and the campaign's reply count sees both.
func TestLiveMultiSenderRepliesCountForEveryMailbox(t *testing.T) {
	f := newRotationFixture(t, 2)
	ctx := context.Background()
	sends := f.sendFromEachMailbox(t)
	adv := f.replyProcessor()

	for _, s := range sends {
		reply := f.storeReply(t, s.mailbox, s)
		if xerr := adv.ProcessIncomingReply(ctx, s.mailbox, reply); xerr != nil {
			t.Fatalf("process reply into %s: %v", s.mailboxEmail, xerr)
		}
		if f.repliedAt(t, s.lead) == nil {
			t.Fatalf("lead %s answered mailbox %s (the one that wrote to it) and was not counted as replied", s.leadEmail, s.mailboxEmail)
		}
	}

	summary, xerr := repository.NewAnalyticsRepository(f.handle).GetCampaignSummary(ctx, f.org, f.campaign, nil)
	if xerr != nil {
		t.Fatalf("campaign summary: %v", xerr)
	}
	if summary.Replies != len(sends) {
		t.Fatalf("campaign summary counts %d replies, want %d (one per rotated mailbox)", summary.Replies, len(sends))
	}
}

// TestLiveMultiSenderReplyWithoutThreadHeadersStillCounts: a client that
// drops In-Reply-To leaves only the sender address, and that has to find the
// lead's latest step whichever mailbox in the pool it wrote from.
func TestLiveMultiSenderReplyWithoutThreadHeadersStillCounts(t *testing.T) {
	f := newRotationFixture(t, 2)
	ctx := context.Background()
	sends := f.sendFromEachMailbox(t)
	adv := f.replyProcessor()

	for _, s := range sends {
		reply := f.storeReply(t, s.mailbox, s)
		reply.InReplyTo = nil
		if _, err := f.pool.Exec(ctx, `UPDATE unibox_emails SET in_reply_to = '{}' WHERE id = $1`, reply.ID); err != nil {
			t.Fatalf("strip thread headers: %v", err)
		}
		if xerr := adv.ProcessIncomingReply(ctx, s.mailbox, reply); xerr != nil {
			t.Fatalf("process reply into %s: %v", s.mailboxEmail, xerr)
		}
		if f.repliedAt(t, s.lead) == nil {
			t.Fatalf("lead %s replied to mailbox %s without thread headers and was not counted", s.leadEmail, s.mailboxEmail)
		}
	}
}
