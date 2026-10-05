// Package crmmode routes the CRM mode endpoints to the provider a workspace
// runs its CRM on (HubSpot or Pipedrive), so the dashboard reads one API
// whichever CRM is connected.
package crmmode

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

// Provider is one connected CRM a workspace can run on.
type Provider interface {
	Name() models.CRMProvider
	// Integration is the integration provider whose connection it runs on.
	Integration() models.IntegrationProvider
	Active(ctx context.Context, orgID uuid.UUID) bool
	// Forget drops this process's cached view of the workspace's mode.
	Forget(orgID uuid.UUID)
	// Left runs when the workspace stops running its CRM on this provider.
	Left(ctx context.Context, orgID, connID uuid.UUID)

	Settings(ctx context.Context, orgID uuid.UUID) (*models.CRMSettings, *errx.Error)
	UpdateSettings(ctx context.Context, orgID uuid.UUID, upd *models.UpdateCRMSettings) (*models.CRMSettings, *errx.Error)
	Metadata(ctx context.Context, orgID uuid.UUID) (*models.CRMMetadata, *errx.Error)
	Owners(ctx context.Context, orgID uuid.UUID) ([]models.CRMOwner, *errx.Error)
	MapOwner(ctx context.Context, orgID uuid.UUID, externalID string, userID *uuid.UUID) *errx.Error
	SyncHealth(ctx context.Context, orgID uuid.UUID) (*models.CRMSyncHealth, *errx.Error)
	RetryFailed(ctx context.Context, orgID uuid.UUID, ids []uuid.UUID) (int64, *errx.Error)
	DiscardFailed(ctx context.Context, orgID uuid.UUID, ids []uuid.UUID) (int64, *errx.Error)
	SyncNow(ctx context.Context, orgID uuid.UUID) *errx.Error
	ContactView(ctx context.Context, orgID, contactID uuid.UUID) (*models.CRMContactView, *errx.Error)
	RefreshContact(ctx context.Context, orgID, contactID uuid.UUID) (*models.CRMContactView, *errx.Error)
	LinkContact(ctx context.Context, orgID, contactID uuid.UUID) (*models.CRMContactView, *errx.Error)
	UpdateContactRecord(ctx context.Context, orgID, contactID uuid.UUID, upd *models.UpdateCRMContact) (*models.CRMContactView, *errx.Error)
	Lists(ctx context.Context, orgID uuid.UUID, query, cursor string, limit int) (*models.CRMListsResult, *errx.Error)
	PreviewImport(ctx context.Context, orgID uuid.UUID, req *models.CRMImportRequest) (*models.CRMImportPreview, *errx.Error)
	Import(ctx context.Context, orgID, userID uuid.UUID, req *models.CRMImportRequest) (*models.CRMImportResult, *errx.Error)
	BackfillPreview(ctx context.Context, orgID uuid.UUID) (*models.CRMBackfillPreview, *errx.Error)
	StartBackfill(ctx context.Context, orgID uuid.UUID, req *models.CRMBackfillRequest) *errx.Error
	EnqueuePush(ctx context.Context, orgID uuid.UUID, objectType string, localID uuid.UUID)
}

// Connections reads integration connections.
type Connections interface {
	GetConnection(ctx context.Context, orgID, id uuid.UUID) (*models.IntegrationConnection, error)
}

// Registry holds every CRM provider the process runs.
type Registry struct {
	repo      repository.CRMProviderRepository
	conns     Connections
	providers []Provider
}

func New(repo repository.CRMProviderRepository, conns Connections, providers ...Provider) *Registry {
	r := &Registry{repo: repo, conns: conns}
	for _, p := range providers {
		if p != nil {
			r.providers = append(r.providers, p)
		}
	}
	return r
}

// Get returns the provider by name, or nil.
func (r *Registry) Get(name models.CRMProvider) Provider {
	for _, p := range r.providers {
		if p.Name() == name {
			return p
		}
	}
	return nil
}

// Mode is the CRM the workspace runs on.
func (r *Registry) Mode(ctx context.Context, orgID uuid.UUID) models.CRMProvider {
	if p := r.active(ctx, orgID); p != nil {
		return p.Name()
	}
	return models.CRMProviderNative
}

func (r *Registry) active(ctx context.Context, orgID uuid.UUID) Provider {
	for _, p := range r.providers {
		if p.Active(ctx, orgID) {
			return p
		}
	}
	return nil
}

// For is the provider running the workspace's CRM, or the refusal a caller
// in Warmbly's own CRM gets.
func (r *Registry) For(ctx context.Context, orgID uuid.UUID) (Provider, *errx.Error) {
	if p := r.active(ctx, orgID); p != nil {
		return p, nil
	}
	return nil, errx.NewWithIdentifier(errx.Conflict, "crm_not_connected",
		"Connect HubSpot or Pipedrive and choose it as your CRM first.")
}

// EnqueuePush queues a record written outside the dashboard for whichever
// CRM the workspace runs on.
func (r *Registry) EnqueuePush(ctx context.Context, orgID uuid.UUID, objectType string, localID uuid.UUID) {
	if p := r.active(ctx, orgID); p != nil {
		p.EnqueuePush(ctx, orgID, objectType, localID)
	}
}

// owner picks the provider that describes a workspace's settings: the one it
// runs on, else the one owning its last connection, else the first.
func (r *Registry) owner(ctx context.Context, orgID uuid.UUID, row *repository.CRMSettingsRow) Provider {
	if row != nil {
		if p := r.Get(row.Provider); p != nil {
			return p
		}
		if row.ConnectionID != nil && r.conns != nil {
			if conn, err := r.conns.GetConnection(ctx, orgID, *row.ConnectionID); err == nil && conn != nil {
				for _, p := range r.providers {
					if p.Integration() == conn.Provider {
						return p
					}
				}
			}
		}
	}
	if len(r.providers) == 0 {
		return nil
	}
	return r.providers[0]
}

var errUnavailable = errx.New(errx.NotImplemented, "CRM providers are not available on this instance")

// Settings returns the workspace's CRM mode and its connected account.
func (r *Registry) Settings(ctx context.Context, orgID uuid.UUID) (*models.CRMSettings, *errx.Error) {
	row, err := r.repo.GetSettings(ctx, orgID)
	if err != nil {
		return nil, errx.InternalError()
	}
	p := r.owner(ctx, orgID, row)
	if p == nil {
		return nil, errUnavailable
	}
	return p.Settings(ctx, orgID)
}

// UpdateSettings switches the workspace's CRM through the provider it is
// switching to (or leaving), and tells the one it left.
func (r *Registry) UpdateSettings(ctx context.Context, orgID uuid.UUID, upd *models.UpdateCRMSettings) (*models.CRMSettings, *errx.Error) {
	before, err := r.repo.GetSettings(ctx, orgID)
	if err != nil {
		return nil, errx.InternalError()
	}
	var p Provider
	if upd.Provider != nil && upd.Provider.External() {
		if p = r.Get(*upd.Provider); p == nil {
			return nil, errx.New(errx.BadRequest, "provider must be native, hubspot or pipedrive")
		}
	} else if upd.Provider != nil && *upd.Provider != models.CRMProviderNative {
		return nil, errx.New(errx.BadRequest, "provider must be native, hubspot or pipedrive")
	} else {
		p = r.owner(ctx, orgID, before)
	}
	if p == nil {
		return nil, errUnavailable
	}
	out, xerr := p.UpdateSettings(ctx, orgID, upd)
	for _, q := range r.providers {
		q.Forget(orgID)
	}
	if xerr != nil {
		return nil, xerr
	}
	if before != nil && before.Provider.External() && before.Provider != out.Provider && before.ConnectionID != nil {
		if left := r.Get(before.Provider); left != nil {
			connID := *before.ConnectionID
			// Tidying up the old provider never holds up the switch.
			go func() {
				lctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				left.Left(lctx, orgID, connID)
			}()
		}
	}
	return out, nil
}
