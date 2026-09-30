package advanced

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

type bounceTaskRepo struct {
	repository.TaskRepository
	task          *repository.Task
	campaignTasks int
}

func (r *bounceTaskRepo) GetTaskByMessageID(context.Context, string) (*repository.Task, error) {
	return r.task, nil
}

func (r *bounceTaskRepo) GetCampaignTask(context.Context, uuid.UUID) (*repository.CampaignTask, error) {
	r.campaignTasks++
	return nil, nil
}

// bounceEmailRepo fails the lookup that comes AFTER the warmup gate, so a test
// can tell "refused as warmup" from "got further and then stopped".
type bounceEmailRepo struct {
	repository.EmailRepository
	reads int
}

func (r *bounceEmailRepo) GetByID(context.Context, uuid.UUID) (*models.Email, *errx.Error) {
	r.reads++
	return nil, errx.ErrNotFound
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
