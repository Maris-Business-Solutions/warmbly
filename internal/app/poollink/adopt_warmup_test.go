package poollink

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/app/email"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

type adoptLinkRepo struct {
	repository.PoolLinkRepository
	enrolled *models.PoolLinkMailbox
}

func (r *adoptLinkRepo) GetMailboxByAccount(context.Context, uuid.UUID) (*models.PoolLinkMailbox, error) {
	return nil, nil
}

func (r *adoptLinkRepo) EnrollMailbox(_ context.Context, m *models.PoolLinkMailbox) error {
	r.enrolled = m
	return nil
}

func (r *adoptLinkRepo) GetMailboxByRemote(context.Context, uuid.UUID, uuid.UUID) (*models.PoolLinkMailbox, error) {
	return r.enrolled, nil
}

type adoptAccounts struct {
	repository.EmailRepository
	acc *models.Email
}

func (r adoptAccounts) GetByID(context.Context, uuid.UUID) (*models.Email, *errx.Error) {
	return r.acc, nil
}

// adoptEmails scopes the lifecycle write by workspace, as the repository does.
type adoptEmails struct {
	email.EmailService
	org     uuid.UUID
	started bool
}

func (e *adoptEmails) SetWarmupLifecycle(_ context.Context, orgID, _, action string) (*models.Email, *errx.Error) {
	if orgID != e.org.String() {
		return nil, errx.ErrNotFound
	}
	e.started = action == "start"
	return &models.Email{}, nil
}

func (e *adoptEmails) LoadAccountOntoWorker(context.Context, uuid.UUID) error { return nil }

// Adopting a workspace mailbox into a linked instance must start its warmup on the cloud.
func TestAdoptStartsWarmupInTheInstanceWorkspace(t *testing.T) {
	org, owner := uuid.New(), uuid.New()
	acc := &models.Email{ID: uuid.New(), OrganizationID: &org, Status: "active", Provider: string(models.InboxProviderGoogle)}
	emails := &adoptEmails{org: org}
	s := &service{repo: &adoptLinkRepo{}, emails: adoptAccounts{acc: acc}, emailSvc: emails}
	inst := &models.PoolLinkInstance{ID: uuid.New(), OrganizationID: org, CreatedBy: &owner}

	if _, xerr := s.Adopt(context.Background(), inst, models.PoolLinkAdoptRequest{RemoteID: uuid.New(), EmailAccountID: acc.ID}); xerr != nil {
		t.Fatalf("Adopt: %v", xerr)
	}
	if !emails.started {
		t.Fatal("warmup was not started in the workspace; the mailbox would sit enrolled and never warm")
	}
}
