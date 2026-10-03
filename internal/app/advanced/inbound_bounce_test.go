package advanced

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

type bounceTaskRepo struct {
	repository.TaskRepository
	task          *repository.Task
	taskErr       error
	campaignTask  *repository.CampaignTask
	campaignErr   error
	campaignTasks int
}

func (r *bounceTaskRepo) GetTaskByMessageID(context.Context, string) (*repository.Task, error) {
	return r.task, r.taskErr
}

func (r *bounceTaskRepo) GetCampaignTask(context.Context, uuid.UUID) (*repository.CampaignTask, error) {
	r.campaignTasks++
	return r.campaignTask, r.campaignErr
}

// bounceEmailRepo fails the lookup that comes AFTER the warmup gate, so a test
// can tell "refused as warmup" from "got further and then stopped". With no
// account or error configured it reports the mailbox as gone (NotFound), the
// genuine-no-match shape; set account/err to drive the other branches.
type bounceEmailRepo struct {
	repository.EmailRepository
	reads   int
	account *models.Email
	err     *errx.Error
}

func (r *bounceEmailRepo) GetByID(context.Context, uuid.UUID) (*models.Email, *errx.Error) {
	r.reads++
	if r.account == nil && r.err == nil {
		return nil, errx.ErrNotFound
	}
	return r.account, r.err
}

// bounceContactRepo drives the contact lookup so a test can separate a technical
// read failure from a pruned contact.
type bounceContactRepo struct {
	repository.ContactRepository
	contact *models.Contact
	err     *errx.Error
}

func (r *bounceContactRepo) GetByID(context.Context, uuid.UUID) (*models.Contact, *errx.Error) {
	return r.contact, r.err
}

// A warmup send's NDR resolves to a task exactly like a campaign send's, because
// warmup stamps tasks.message_id too. Attributing it suppressed a pool
// partner's address in the customer's list and recorded a bounce against their
// deliverability, where it fed the breaker.
func TestRecordInboundBounceRefusesWarmupSends(t *testing.T) {
	mailbox := uuid.New()
	tasks := &bounceTaskRepo{task: &repository.Task{
		ID:             uuid.New(),
		TaskType:       models.TaskTypeWarmup,
		EmailAccountID: mailbox,
		MessageID:      "<warm-1@sender.test>",
	}}
	emails := &bounceEmailRepo{}
	s := &service{taskRepo: tasks, emailRepo: emails}

	if err := s.RecordInboundBounce(context.Background(), mailbox, "warm-1@sender.test", "partner@pool.test", "550 mailbox unavailable"); err != nil {
		t.Fatalf("a warmup NDR should be dropped quietly, got %v", err)
	}
	if emails.reads != 0 {
		t.Fatal("a warmup NDR was carried past the gate and into deliverability ingest")
	}
	if tasks.campaignTasks != 0 {
		t.Fatal("a warmup NDR should not be looked up as a campaign send")
	}
}

// The same NDR for a real send still has to be attributed, or the gate has
// bought silence at the cost of campaign bounce tracking.
func TestRecordInboundBounceStillAttributesRealSends(t *testing.T) {
	mailbox := uuid.New()
	tasks := &bounceTaskRepo{task: &repository.Task{
		ID:             uuid.New(),
		TaskType:       "campaign",
		EmailAccountID: mailbox,
		MessageID:      "<camp-1@sender.test>",
	}}
	emails := &bounceEmailRepo{}
	s := &service{taskRepo: tasks, emailRepo: emails}

	// The mailbox lookup refuses, so ingest stops there; reaching it at all is
	// what this asserts.
	_ = s.RecordInboundBounce(context.Background(), mailbox, "camp-1@sender.test", "lead@prospect.test", "550 no such user")
	if emails.reads == 0 {
		t.Fatal("a campaign NDR was dropped by the warmup gate")
	}
}

type copyBounceProgress struct {
	repository.CampaignProgressRepository
	copies map[string]uuid.UUID
}

func (r copyBounceProgress) MarkLeadCCBounced(_ context.Context, _, _ uuid.UUID, address string) (*uuid.UUID, error) {
	if id, ok := r.copies[address]; ok {
		return &id, nil
	}
	return nil, nil
}

type copyBounceCampaigns struct {
	repository.CampaignRepository
	cc, bcc []string
}

func (r copyBounceCampaigns) GetByID(_ context.Context, id uuid.UUID) (*models.Campaign, error) {
	return &models.Campaign{ID: id, CC: r.cc, BCC: r.bcc}, nil
}

func campaignSendTask(mailbox uuid.UUID) *repository.Task {
	return &repository.Task{
		ID:             uuid.New(),
		TaskType:       "campaign",
		EmailAccountID: mailbox,
		MessageID:      "<camp-1@sender.test>",
	}
}

// A technical failure resolving the original Message-ID (connection reset,
// statement timeout) is not evidence the id is unknown. It must surface as an
// error so the event is redelivered, not be swallowed as a no-match.
func TestRecordInboundBounceRetriesOnTaskLookupFailure(t *testing.T) {
	tasks := &bounceTaskRepo{taskErr: errors.New("connection reset")}
	emails := &bounceEmailRepo{}
	s := &service{taskRepo: tasks, emailRepo: emails}

	if err := s.RecordInboundBounce(context.Background(), uuid.New(), "camp-1@sender.test", "lead@prospect.test", "550"); err == nil {
		t.Fatal("a technical task lookup failure must surface an error for retry")
	}
	if emails.reads != 0 {
		t.Fatal("a failed task lookup should not reach the mailbox lookup")
	}
}

// A genuine no-match (no task carries the id: a non-Warmbly or pruned send) is a
// decision, not a failure: it acks with nil so the bus does not loop on it.
func TestRecordInboundBounceAcksGenuineNoMatch(t *testing.T) {
	tasks := &bounceTaskRepo{task: nil} // (nil, nil): no matching row
	emails := &bounceEmailRepo{}
	s := &service{taskRepo: tasks, emailRepo: emails}

	if err := s.RecordInboundBounce(context.Background(), uuid.New(), "nope@sender.test", "lead@prospect.test", "550"); err != nil {
		t.Fatalf("a genuine no-match should ack with nil, got %v", err)
	}
	if emails.reads != 0 {
		t.Fatal("a no-match should not reach the mailbox lookup")
	}
}

// A technical read of the sending mailbox retries; a mailbox that is genuinely
// gone (NotFound) acks with nil.
func TestRecordInboundBounceClassifiesMailboxLookup(t *testing.T) {
	mailbox := uuid.New()

	techEmails := &bounceEmailRepo{err: errx.InternalError()}
	s := &service{taskRepo: &bounceTaskRepo{task: campaignSendTask(mailbox)}, emailRepo: techEmails}
	if err := s.RecordInboundBounce(context.Background(), mailbox, "camp-1@sender.test", "lead@prospect.test", "550"); err == nil {
		t.Fatal("a technical mailbox read failure must surface an error for retry")
	}

	goneEmails := &bounceEmailRepo{err: errx.ErrNotFound}
	s = &service{taskRepo: &bounceTaskRepo{task: campaignSendTask(mailbox)}, emailRepo: goneEmails}
	if err := s.RecordInboundBounce(context.Background(), mailbox, "camp-1@sender.test", "lead@prospect.test", "550"); err != nil {
		t.Fatalf("a mailbox that no longer exists should ack with nil, got %v", err)
	}
}

// A technical failure resolving the campaign send is returned for retry rather
// than ingesting the bounce half-attributed.
func TestRecordInboundBounceRetriesOnCampaignTaskFailure(t *testing.T) {
	mailbox, org := uuid.New(), uuid.New()
	emails := &bounceEmailRepo{account: &models.Email{ID: mailbox, OrganizationID: &org}}
	tasks := &bounceTaskRepo{task: campaignSendTask(mailbox), campaignErr: errors.New("statement timeout")}
	s := &service{taskRepo: tasks, emailRepo: emails}

	if err := s.RecordInboundBounce(context.Background(), mailbox, "lead@prospect.test", "lead@prospect.test", "550"); err == nil {
		t.Fatal("a technical campaign-task lookup failure must surface an error for retry")
	}
}

// A technical failure reading the contact is returned for retry.
func TestRecordInboundBounceRetriesOnContactFailure(t *testing.T) {
	mailbox, org, contactID, campaignID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	task := campaignSendTask(mailbox)
	emails := &bounceEmailRepo{account: &models.Email{ID: mailbox, OrganizationID: &org}}
	tasks := &bounceTaskRepo{task: task, campaignTask: &repository.CampaignTask{TaskID: task.ID, CampaignID: &campaignID, ContactID: &contactID}}
	contacts := &bounceContactRepo{err: errx.InternalError()}
	s := &service{taskRepo: tasks, emailRepo: emails, contactRepo: contacts}

	if err := s.RecordInboundBounce(context.Background(), mailbox, "camp-1@sender.test", "", "550"); err == nil {
		t.Fatal("a technical contact read failure must surface an error for retry")
	}
}

// Complaint ingestion classifies the same way: a technical task lookup failure
// retries, a genuine no-match acks.
func TestRecordInboundComplaintRetriesOnTaskLookupFailure(t *testing.T) {
	tasks := &bounceTaskRepo{taskErr: errors.New("connection reset")}
	emails := &bounceEmailRepo{}
	s := &service{taskRepo: tasks, emailRepo: emails}

	if err := s.RecordInboundComplaint(context.Background(), uuid.New(), "camp-1@sender.test", "lead@prospect.test", "fbl"); err == nil {
		t.Fatal("a technical task lookup failure must surface an error for retry")
	}
	if emails.reads != 0 {
		t.Fatal("a failed task lookup should not reach the mailbox lookup")
	}
}

func TestRecordInboundComplaintAcksGenuineNoMatch(t *testing.T) {
	tasks := &bounceTaskRepo{task: nil}
	emails := &bounceEmailRepo{}
	s := &service{taskRepo: tasks, emailRepo: emails}

	if err := s.RecordInboundComplaint(context.Background(), uuid.New(), "nope@sender.test", "lead@prospect.test", "fbl"); err != nil {
		t.Fatalf("a genuine no-match should ack with nil, got %v", err)
	}
}

// A technical contact read retries; a contact genuinely pruned (NotFound) acks
// with nil and attributes nothing, preserving the resolved-send-only rule.
func TestRecordInboundComplaintClassifiesContactLookup(t *testing.T) {
	mailbox, org, contactID, campaignID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	task := campaignSendTask(mailbox)
	mkTasks := func() *bounceTaskRepo {
		return &bounceTaskRepo{task: task, campaignTask: &repository.CampaignTask{TaskID: task.ID, CampaignID: &campaignID, ContactID: &contactID}}
	}
	mkEmails := func() *bounceEmailRepo {
		return &bounceEmailRepo{account: &models.Email{ID: mailbox, OrganizationID: &org}}
	}

	s := &service{taskRepo: mkTasks(), emailRepo: mkEmails(), contactRepo: &bounceContactRepo{err: errx.InternalError()}}
	if err := s.RecordInboundComplaint(context.Background(), mailbox, "camp-1@sender.test", "", "fbl"); err == nil {
		t.Fatal("a technical contact read failure must surface an error for retry")
	}

	s = &service{taskRepo: mkTasks(), emailRepo: mkEmails(), contactRepo: &bounceContactRepo{err: errx.ErrNotFound}}
	if err := s.RecordInboundComplaint(context.Background(), mailbox, "camp-1@sender.test", "", "fbl"); err != nil {
		t.Fatalf("a pruned contact should ack with nil, got %v", err)
	}
}

// A DSN naming someone copied on the send bounces that copy, not the lead; an
// address nobody copied (a forward, an alias) stays the lead's as before.
func TestCopyBounceOwnerTellsACopyFromTheLead(t *testing.T) {
	lead, copied := uuid.New(), uuid.New()
	s := &service{
		campaignProgressRepo: copyBounceProgress{copies: map[string]uuid.UUID{"jonas@acme.test": copied}},
		campaignRepo:         copyBounceCampaigns{cc: []string{"Boss <boss@acme.test>"}, bcc: []string{"crm@acme.test"}},
	}
	for _, tc := range []struct {
		address string
		owner   *uuid.UUID
		isCopy  bool
	}{
		{"jonas@acme.test", &copied, true},
		{"BOSS@acme.test", nil, true},
		{"crm@acme.test", nil, true},
		{"forwarded@elsewhere.test", &lead, false},
	} {
		owner, isCopy := s.copyBounceOwner(context.Background(), uuid.New(), lead, tc.address)
		if isCopy != tc.isCopy || (owner == nil) != (tc.owner == nil) || (owner != nil && *owner != *tc.owner) {
			t.Errorf("%s: owner %v copy %v, want %v %v", tc.address, owner, isCopy, tc.owner, tc.isCopy)
		}
	}
}
