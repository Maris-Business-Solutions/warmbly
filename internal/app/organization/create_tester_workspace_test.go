package organization

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/models"
)

type testerCreateRepo struct {
	createRepo
	grantErr error
	deleted  bool
	until    time.Time
	actor    uuid.UUID
}

func (r *testerCreateRepo) ProvisionTesterWorkspace(_ context.Context, _, _, actor uuid.UUID, _ string, until time.Time) error {
	r.actor, r.until = actor, until
	return r.grantErr
}

func (r *testerCreateRepo) Delete(context.Context, uuid.UUID) error {
	r.deleted = true
	return nil
}

func TestCreateTesterWorkspaceRequiresACompleteGrant(t *testing.T) {
	for _, fail := range []bool{false, true} {
		r := &testerCreateRepo{}
		if fail {
			r.grantErr = errors.New("credit ledger unavailable")
		}
		s := &organizationService{orgRepo: r, userRepo: createUsers{}}
		actor := uuid.New()
		until := time.Now().Add(time.Hour)
		org, xerr := s.CreateTesterWorkspace(context.Background(), uuid.New(), actor, "Reviewer workspace", "OAuth review", until)
		if fail {
			if xerr == nil || org != nil || !r.deleted {
				t.Fatalf("failed grant left a usable workspace: %+v, %v, deleted=%v", org, xerr, r.deleted)
			}
		} else if xerr != nil || org.Category != models.OrganizationCategoryTest || !r.until.Equal(until) || r.actor != actor {
			t.Fatalf("tester grant not complete: %+v, %v", org, xerr)
		}
	}
}
