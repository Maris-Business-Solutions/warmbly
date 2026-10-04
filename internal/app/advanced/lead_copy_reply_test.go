package advanced

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

type copyReplyAdvancedRepo struct {
	*incomingReplyAdvancedRepo
	suppressed []string
}

func (r *copyReplyAdvancedRepo) UpsertSuppressedRecipient(_ context.Context, s *models.SuppressedRecipient) error {
	r.suppressed = append(r.suppressed, strings.ToLower(s.Email))
	return nil
}

type copyReplyContactRepo struct {
	incomingReplyContactRepo
}

func (copyReplyContactRepo) SetSubscribedByEmail(context.Context, uuid.UUID, string, bool) error {
	return nil
}

type copyReplyProgressRepo struct {
	*incomingReplyProgressRepo
	heldEverywhere int
}

func (r *copyReplyProgressRepo) HoldLeadEverywhere(context.Context, uuid.UUID, *time.Time, string, string) ([]uuid.UUID, error) {
	r.heldEverywhere++
	return nil, nil
}

// newCopyReplyService is the incoming-reply harness with the lead copying
// jonas@acme.test, answering in the lead's thread.
func newCopyReplyService(t *testing.T) (*service, *copyReplyProgressRepo, *copyReplyAdvancedRepo, uuid.UUID) {
	t.Helper()
	orgID, accountID, leadID := uuid.New(), uuid.New(), uuid.New()
	account := &models.Email{ID: accountID, OrganizationID: &orgID, Email: "sender@example.test"}
	svc, progress := newIncomingReplyService(account, nil, leadID)
	progress.copies = []models.CampaignLeadCC{{ContactID: uuid.New(), Email: "jonas@acme.test", Status: models.LeadCCStatusActive}}
	adv := &copyReplyAdvancedRepo{incomingReplyAdvancedRepo: progress.advanced}
	wrapped := &copyReplyProgressRepo{incomingReplyProgressRepo: progress}
	svc.repo = adv
	svc.campaignProgressRepo = wrapped
	svc.contactRepo = copyReplyContactRepo{svc.contactRepo.(incomingReplyContactRepo)}
	return svc, wrapped, adv, accountID
}

func copyReply(accountID uuid.UUID, subject, body string, inReplyTo []string) *models.EmailMessageStoreData {
	return &models.EmailMessageStoreData{
		ID: uuid.New(), EmailID: accountID, Folder: models.FolderInbox,
		FromAddr:  []string{"Jonas <jonas@acme.test>"},
		ToAddr:    []string{"sender@example.test"},
		InReplyTo: inReplyTo,
		Subject:   subject,
		Snippet:   body,
		BodyText:  body,
	}
}

// A copy asking to stop is about the copy: the lead they were copied on is
// not suppressed with them.
func TestCopyOptOutSuppressesOnlyTheCopy(t *testing.T) {
	svc, _, adv, accountID := newCopyReplyService(t)
	if xerr := svc.ProcessIncomingReply(context.Background(), accountID,
		copyReply(accountID, "Re: Hello", "Please remove me from your list.", []string{"<opener@example.test>"})); xerr != nil {
		t.Fatal(xerr)
	}
	if len(adv.suppressed) != 1 || adv.suppressed[0] != "jonas@acme.test" {
		t.Fatalf("suppressed %v, want only the copy", adv.suppressed)
	}
}

// A copy's away message says nothing about the lead's desk.
func TestCopyOutOfOfficeDoesNotHoldTheLead(t *testing.T) {
	svc, progress, _, accountID := newCopyReplyService(t)
	if xerr := svc.ProcessIncomingReply(context.Background(), accountID,
		copyReply(accountID, "Automatic reply: Hello", "I am out of the office until Monday.", []string{"<opener@example.test>"})); xerr != nil {
		t.Fatal(xerr)
	}
	if progress.heldEverywhere != 0 {
		t.Fatalf("the lead was held %d times for a copy's away message", progress.heldEverywhere)
	}
}

// A copy answering further down the thread names no message of ours; the
// mailbox that wrote to the lead ties it back, and it is the lead's reply.
func TestCopyReplyDownThreadCountsForTheLead(t *testing.T) {
	svc, progress, _, accountID := newCopyReplyService(t)
	copyContact := &models.Contact{ID: uuid.New(), Email: "jonas@acme.test"}
	svc.taskRepo = incomingReplyTaskRepo{}
	cr := svc.contactRepo.(copyReplyContactRepo)
	cr.senderContact = copyContact
	svc.contactRepo = cr
	progress.copiedLead = &repository.CopiedLeadRef{CampaignID: uuid.New(), ContactID: uuid.New(), SequenceID: uuid.New()}

	if xerr := svc.ProcessIncomingReply(context.Background(), accountID,
		copyReply(accountID, "Re: Hello", "Sounds good, let's talk Tuesday.", []string{"<leads-own-reply@acme.test>"})); xerr != nil {
		t.Fatal(xerr)
	}
	if progress.replied != 1 {
		t.Fatalf("RecordEmailReplied calls = %d, want the copy's reply counted for the lead", progress.replied)
	}
}

// The link in a copied message cannot say who used it, so it opts out the
// lead and everyone copied; a sequence action is about the lead alone.
func TestUnsubscribeLinkOptsOutTheLeadsCopies(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(s *service, org, campaign, lead uuid.UUID) *errx.Error
		want []string
	}{
		{"link", func(s *service, org, campaign, lead uuid.UUID) *errx.Error {
			return s.UnsubscribeFromLink(context.Background(), org, campaign, lead, "one_click")
		}, []string{"task-contact@example.test", "jonas@acme.test"}},
		{"sequence action", func(s *service, org, campaign, lead uuid.UUID) *errx.Error {
			return s.Unsubscribe(context.Background(), org, campaign, lead)
		}, []string{"task-contact@example.test"}},
	} {
		svc, _, adv, _ := newCopyReplyService(t)
		campaign := svc.campaignRepo.(incomingReplyCampaignRepo).campaign
		lead := svc.contactRepo.(copyReplyContactRepo).taskContact.ID
		if xerr := tc.run(svc, *campaign.OrganizationID, campaign.ID, lead); xerr != nil {
			t.Fatalf("%s: %v", tc.name, xerr)
		}
		if strings.Join(adv.suppressed, ",") != strings.Join(tc.want, ",") {
			t.Fatalf("%s: suppressed %v, want %v", tc.name, adv.suppressed, tc.want)
		}
	}
}

// An unreadable copy list is an error, never "not a copy": guessing would
// suppress or hold the lead for what a copy did.
func TestCopyLookupFailureIsNotReadAsTheLead(t *testing.T) {
	svc, progress, adv, accountID := newCopyReplyService(t)
	progress.copiesErr = errors.New("database unavailable")
	if xerr := svc.ProcessIncomingReply(context.Background(), accountID,
		copyReply(accountID, "Re: Hello", "Please remove me from your list.", []string{"<opener@example.test>"})); xerr == nil {
		t.Fatal("a failed copy lookup was processed as if the sender were not a copy")
	}
	if len(adv.suppressed) != 0 {
		t.Fatalf("suppressed %v on a failed lookup", adv.suppressed)
	}
}

// A fresh message from a copy is not a reply to anything, so it credits no lead.
func TestCopyFreshMessageCreditsNoLead(t *testing.T) {
	svc, progress, _, accountID := newCopyReplyService(t)
	svc.taskRepo = incomingReplyTaskRepo{}
	cr := svc.contactRepo.(copyReplyContactRepo)
	cr.senderContact = &models.Contact{ID: uuid.New(), Email: "jonas@acme.test"}
	svc.contactRepo = cr
	progress.copiedLead = &repository.CopiedLeadRef{CampaignID: uuid.New(), ContactID: uuid.New(), SequenceID: uuid.New()}

	if xerr := svc.ProcessIncomingReply(context.Background(), accountID,
		copyReply(accountID, "Quick question", "Unrelated: are you at the fair next week?", nil)); xerr != nil {
		t.Fatal(xerr)
	}
	if progress.replied != 0 {
		t.Fatalf("RecordEmailReplied calls = %d, want none for a message that replies to nothing", progress.replied)
	}
}

// The opt-out is not acknowledged while a copy is still sendable.
func TestUnsubscribeFailsWhenCopiesCannotBeRead(t *testing.T) {
	svc, progress, _, _ := newCopyReplyService(t)
	progress.copiesErr = errors.New("database unavailable")
	campaign := svc.campaignRepo.(incomingReplyCampaignRepo).campaign
	lead := svc.contactRepo.(copyReplyContactRepo).taskContact.ID
	if xerr := svc.UnsubscribeFromLink(context.Background(), *campaign.OrganizationID, campaign.ID, lead, "link"); xerr == nil {
		t.Fatal("the unsubscribe succeeded without reaching the lead's copies")
	}
}
