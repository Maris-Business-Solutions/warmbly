package tasks

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/app/advanced"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

type copiesAdvanced struct {
	advanced.Service
	suppressed map[string]bool
}

func (f copiesAdvanced) ShouldSuppressRecipient(_ context.Context, _ uuid.UUID, recipient string) (bool, string, *errx.Error) {
	return f.suppressed[strings.ToLower(recipient)], "", nil
}

type copiesProgress struct {
	repository.CampaignProgressRepository
	cc []models.CampaignLeadCC
}

func (f copiesProgress) ListLeadCC(context.Context, uuid.UUID, uuid.UUID) ([]models.CampaignLeadCC, error) {
	return f.cc, nil
}

// A copy never reaches someone the lead's own email could not: suppressed
// campaign copies, the lead's own address, repeats and lead copies the status
// already refused are all left off.
func TestCampaignCopiesFiltersEveryCopy(t *testing.T) {
	s := &tasksService{
		advanced: copiesAdvanced{suppressed: map[string]bool{"gone@acme.test": true}},
		campaignProgressRepo: copiesProgress{cc: []models.CampaignLeadCC{
			{Email: "jonas@acme.test", Status: models.LeadCCStatusActive},
			{Email: "bounced@acme.test", Status: models.LeadCCStatusBounced},
			{Email: "Boss@acme.test", Status: models.LeadCCStatusActive},
			{Email: "ana@acme.test", Status: models.LeadCCStatusActive},
		}},
	}
	campaign := &models.Campaign{
		ID:  uuid.New(),
		CC:  []string{"Boss <boss@acme.test>", "gone@acme.test", "ANA@acme.test"},
		BCC: []string{"crm@acme.test", "boss@acme.test"},
	}
	contact := &models.Contact{ID: uuid.New(), Email: "ana@acme.test"}

	cc, bcc, err := s.campaignCopies(context.Background(), uuid.New(), campaign, contact)
	if err != nil {
		t.Fatalf("campaignCopies: %v", err)
	}
	if want := []string{"Boss <boss@acme.test>", "jonas@acme.test"}; !reflect.DeepEqual(cc, want) {
		t.Fatalf("cc = %v, want %v", cc, want)
	}
	if want := []string{"crm@acme.test"}; !reflect.DeepEqual(bcc, want) {
		t.Fatalf("bcc = %v, want %v", bcc, want)
	}
}
