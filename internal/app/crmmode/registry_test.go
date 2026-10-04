package crmmode

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

type fakeRepo struct {
	repository.CRMProviderRepository
	row *repository.CRMSettingsRow
}

func (f *fakeRepo) GetSettings(context.Context, uuid.UUID) (*repository.CRMSettingsRow, error) {
	return f.row, nil
}

type fakeProvider struct {
	Provider
	name   models.CRMProvider
	repo   *fakeRepo
	mu     sync.Mutex
	left   chan uuid.UUID
	forgot int
}

func (p *fakeProvider) Name() models.CRMProvider { return p.name }

func (p *fakeProvider) Integration() models.IntegrationProvider {
	return models.IntegrationProvider(p.name)
}

func (p *fakeProvider) Active(context.Context, uuid.UUID) bool {
	return p.repo.row != nil && p.repo.row.Provider == p.name
}

func (p *fakeProvider) Forget(uuid.UUID) {
	p.mu.Lock()
	p.forgot++
	p.mu.Unlock()
}

func (p *fakeProvider) Left(_ context.Context, _ uuid.UUID, connID uuid.UUID) { p.left <- connID }

func (p *fakeProvider) UpdateSettings(_ context.Context, orgID uuid.UUID, upd *models.UpdateCRMSettings) (*models.CRMSettings, *errx.Error) {
	next := models.CRMProviderNative
	if upd.Provider != nil {
		next = *upd.Provider
	}
	conn := upd.ConnectionID
	p.repo.row = &repository.CRMSettingsRow{OrganizationID: orgID, Provider: next, ConnectionID: conn}
	return &models.CRMSettings{OrganizationID: orgID, Provider: next, ConnectionID: conn}, nil
}

func ptr[T any](v T) *T { return &v }

func TestRegistryRoutesAndTidies(t *testing.T) {
	org := uuid.New()
	hubConn, pdConn := uuid.New(), uuid.New()
	repo := &fakeRepo{row: &repository.CRMSettingsRow{OrganizationID: org, Provider: models.CRMProviderHubSpot, ConnectionID: &hubConn}}
	hub := &fakeProvider{name: models.CRMProviderHubSpot, repo: repo, left: make(chan uuid.UUID, 1)}
	pd := &fakeProvider{name: models.CRMProviderPipedrive, repo: repo, left: make(chan uuid.UUID, 1)}
	r := New(repo, nil, hub, pd)

	if r.Mode(context.Background(), org) != models.CRMProviderHubSpot {
		t.Fatal("the workspace runs on HubSpot")
	}
	out, xerr := r.UpdateSettings(context.Background(), org, &models.UpdateCRMSettings{
		Provider: ptr(models.CRMProviderPipedrive), ConnectionID: &pdConn,
	})
	if xerr != nil || out.Provider != models.CRMProviderPipedrive {
		t.Fatalf("switching to Pipedrive goes through Pipedrive, got %v %v", out, xerr)
	}
	select {
	case got := <-hub.left:
		if got != hubConn {
			t.Fatalf("HubSpot is told which connection it left, got %s", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the provider a workspace leaves is told")
	}
	if hub.forgot == 0 || pd.forgot == 0 {
		t.Fatal("every provider drops its cached view after a switch")
	}
	if p, xerr := r.For(context.Background(), org); xerr != nil || p.Name() != models.CRMProviderPipedrive {
		t.Fatal("the CRM routes resolve to Pipedrive now")
	}

	if _, xerr := r.UpdateSettings(context.Background(), org, &models.UpdateCRMSettings{Provider: ptr(models.CRMProviderNative)}); xerr != nil {
		t.Fatal(xerr)
	}
	select {
	case <-pd.left:
	case <-time.After(2 * time.Second):
		t.Fatal("going back to Warmbly's CRM tells Pipedrive")
	}
	if _, xerr := r.For(context.Background(), org); xerr == nil || xerr.Identifier != "crm_not_connected" {
		t.Fatal("Warmbly's own CRM has no provider to route to")
	}
	if _, xerr := r.UpdateSettings(context.Background(), org, &models.UpdateCRMSettings{Provider: ptr(models.CRMProvider("salesforce"))}); xerr == nil {
		t.Fatal("an unknown provider is refused")
	}
}
