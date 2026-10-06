package organization

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

type testerSampleRepo struct {
	repository.OrganizationRepository
	orgID, actor uuid.UUID
	ip, agent    string
	result       *models.TesterSampleData
	err          error
}

func (r *testerSampleRepo) SeedTesterWorkspace(_ context.Context, orgID, actor uuid.UUID, ip, agent string) (*models.TesterSampleData, error) {
	r.orgID, r.actor, r.ip, r.agent = orgID, actor, ip, agent
	return r.result, r.err
}

func TestSeedTesterWorkspacePreservesResultAndAuditActor(t *testing.T) {
	orgID, actor := uuid.New(), uuid.New()
	for _, created := range []bool{true, false} {
		r := &testerSampleRepo{result: &models.TesterSampleData{OrganizationID: orgID, Created: created, SeededAt: time.Now()}}
		s := &organizationService{orgRepo: r}
		result, xerr := s.SeedTesterWorkspace(context.Background(), orgID, actor, "127.0.0.1", "reviewer")
		if xerr != nil || result != r.result || r.orgID != orgID || r.actor != actor || r.ip != "127.0.0.1" || r.agent != "reviewer" {
			t.Fatalf("sample result or attribution changed: %+v, %v", result, xerr)
		}
	}
}

func TestSeedTesterWorkspaceMapsRepositoryErrors(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code errx.Code
	}{
		{repository.ErrTesterWorkspaceInactive, errx.BadRequest},
		{errors.New("database unavailable"), errx.Internal},
	} {
		s := &organizationService{orgRepo: &testerSampleRepo{err: tc.err}}
		result, xerr := s.SeedTesterWorkspace(context.Background(), uuid.New(), uuid.New(), "", "")
		if result != nil || xerr == nil || xerr.Code != tc.code {
			t.Fatalf("unexpected sample error: %+v, %v", result, xerr)
		}
	}
}
