package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/app/advanced"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

type copyContactRepo struct {
	repository.ContactRepository
	email string
}

func (r copyContactRepo) GetByID(_ context.Context, id uuid.UUID) (*models.Contact, *errx.Error) {
	return &models.Contact{ID: id, Email: r.email}, nil
}

type copyCampaignRepo struct {
	repository.CampaignRepository
	org uuid.UUID
}

func (r copyCampaignRepo) GetByID(_ context.Context, id uuid.UUID) (*models.Campaign, error) {
	return &models.Campaign{ID: id, OrganizationID: &r.org, Status: "active"}, nil
}

func (copyCampaignRepo) DecrementCampaignDailySend(context.Context, uuid.UUID, time.Time, bool) error {
	return nil
}

type copyProgressRepo struct {
	repository.CampaignProgressRepository
	copyID  uuid.UUID
	bounced []string
	counted []bool
}

func (r *copyProgressRepo) RecordSendFailure(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string) (int, bool, bool, error) {
	r.counted = append(r.counted, true)
	return 1, false, true, nil
}

func (r *copyProgressRepo) WalkBackSend(_ context.Context, _, _, _ uuid.UUID, _ string, count bool) (int, bool, bool, error) {
	r.counted = append(r.counted, count)
	return 0, false, true, nil
}

func (*copyProgressRepo) HasSentSteps(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

func (r *copyProgressRepo) MarkLeadCCBounced(_ context.Context, _, _ uuid.UUID, address string) (*uuid.UUID, error) {
	r.bounced = append(r.bounced, address)
	return &r.copyID, nil
}

type copyAdvanced struct {
	advanced.Service
	events []*models.IngestDeliverabilityEventRequest
}

func (a *copyAdvanced) IngestDeliverabilityEvent(_ context.Context, _ uuid.UUID, req *models.IngestDeliverabilityEventRequest) *errx.Error {
	a.events = append(a.events, req)
	return nil
}

// A copy the server refused at RCPT bounces the copy, never the lead: no
// address evidence against the lead, and the bounce event names the copy.
func TestRefusedCopyIsNotTheLeadsBounce(t *testing.T) {
	for _, tc := range []struct {
		name     string
		refused  string
		wantCopy bool
	}{
		{"a copy", "Jonas <jonas@acme.test>", true},
		{"the lead", "ana@acme.test", false},
	} {
		campaign, lead, step, taskID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
		progress := &copyProgressRepo{copyID: uuid.New()}
		adv := &copyAdvanced{}
		ev := &recordingEvidence{}
		s := &JobsService{
			TaskRepo: &evidenceTaskRepo{
				task: &repository.Task{ID: taskID, TaskType: "campaign", EmailAccountID: uuid.New(), Status: "completed"},
				ct:   &repository.CampaignTask{TaskID: taskID, CampaignID: &campaign, ContactID: &lead, SequenceID: &step},
			},
			CampaignRepo:         copyCampaignRepo{org: uuid.New()},
			CampaignProgressRepo: progress,
			ContactRepo:          copyContactRepo{email: "ana@acme.test"},
			AdvancedService:      adv,
			Evidence:             ev,
		}
		err := s.HandleEmailFailed(context.Background(), models.SendEmailResult{
			TaskID: taskID,
			Error: &models.EmailSendError{
				Code:      string(errx.MailErrorCodeRecipientRejected),
				Message:   `The mail server rejected the recipient: 550 "5.1.1 no such user"`,
				Recipient: tc.refused,
			},
		})
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if len(adv.events) != 1 {
			t.Fatalf("%s: %d bounce events, want 1", tc.name, len(adv.events))
		}
		got := adv.events[0]
		if tc.wantCopy {
			if len(ev.kinds) != 0 {
				t.Fatalf("%s: evidence %v recorded against the lead", tc.name, ev.kinds)
			}
			if got.RecipientEmail != "jonas@acme.test" || got.ContactID == nil || *got.ContactID != progress.copyID {
				t.Fatalf("%s: bounce event %+v, want it on the copy", tc.name, got)
			}
			if len(progress.bounced) != 1 {
				t.Fatalf("%s: copy marked bounced %v times, want once", tc.name, progress.bounced)
			}
			// The retry leaves the copy off, so the lead is not charged an attempt.
			if len(progress.counted) != 1 || progress.counted[0] {
				t.Fatalf("%s: walk-backs %v, want one that counts no attempt", tc.name, progress.counted)
			}
			continue
		}
		if len(progress.counted) != 1 || !progress.counted[0] {
			t.Fatalf("%s: walk-backs %v, want the lead's attempt counted", tc.name, progress.counted)
		}
		if got.RecipientEmail != "ana@acme.test" || got.ContactID == nil || *got.ContactID != lead || len(progress.bounced) != 0 {
			t.Fatalf("%s: bounce event %+v (copies marked %v), want the lead's own bounce", tc.name, got, progress.bounced)
		}
	}
}
