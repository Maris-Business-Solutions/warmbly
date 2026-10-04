package hubspot

import (
	"context"
	"encoding/json"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

func jsonUnmarshal(raw []byte, v any) error { return json.Unmarshal(raw, v) }

// requiredScopes are what full CRM mode needs; a connection made before CRM
// mode existed has only the contact scopes and is asked to reconnect.
var requiredScopes = []string{
	"crm.objects.contacts.write", "crm.objects.companies.write", "crm.objects.deals.write",
	"crm.objects.owners.read", "crm.schemas.contacts.write",
}

// Settings returns the workspace's CRM mode with the connected account.
func (s *Service) Settings(ctx context.Context, orgID uuid.UUID) (*models.CRMSettings, *errx.Error) {
	row, err := s.d.Repo.GetSettings(ctx, orgID)
	if err != nil {
		return nil, errx.InternalError()
	}
	out := &models.CRMSettings{OrganizationID: orgID, Provider: models.CRMProviderNative, Config: models.DefaultCRMProviderConfig()}
	if row != nil && row.Provider == models.CRMProviderNative && !row.Config.BelongsTo(provider) {
		row.Config = models.DefaultCRMProviderConfig()
	}
	if row != nil {
		out.Provider = row.Provider
		out.ConnectionID = row.ConnectionID
		out.Config = row.Config
		out.SetupCompletedAt = row.SetupCompletedAt
		out.UpdatedAt = row.UpdatedAt
	}
	if out.ConnectionID != nil {
		if conn, cerr := s.d.Tokens.GetConnection(ctx, orgID, *out.ConnectionID); cerr == nil && conn != nil {
			out.Account = accountView(conn)
		}
	}
	if out.Provider == provider && out.Account == nil {
		// The HubSpot connection was removed: the CRM is Warmbly's again.
		out.Provider = models.CRMProviderNative
	}
	return out, nil
}

func accountView(conn *models.IntegrationConnection) *models.CRMAccount {
	ui := displayString(conn.DisplayFields, "ui_domain")
	if ui == "" {
		ui = "app.hubspot.com"
	}
	acct := &models.CRMAccount{
		ExternalID: conn.ExternalAccountID,
		Name:       conn.ExternalAccountName,
		Status:     string(conn.Status),
		Health:     conn.Health,
	}
	if conn.ExternalAccountID != "" {
		acct.AppURL = "https://" + ui + "/contacts/" + conn.ExternalAccountID
	}
	for _, sc := range requiredScopes {
		if !slices.Contains(conn.GrantedScopes, sc) {
			acct.MissingScopes = append(acct.MissingScopes, sc)
		}
	}
	return acct
}

// UpdateSettings switches the workspace's CRM and stores its choices. Choosing
// HubSpot prepares the portal (the Warmbly property group) and starts the
// first pull, so the dashboard has data by the time the wizard closes.
func (s *Service) UpdateSettings(ctx context.Context, orgID uuid.UUID, upd *models.UpdateCRMSettings) (*models.CRMSettings, *errx.Error) {
	row, err := s.d.Repo.GetSettings(ctx, orgID)
	if err != nil {
		return nil, errx.InternalError()
	}
	if row == nil {
		row = &repository.CRMSettingsRow{OrganizationID: orgID, Provider: models.CRMProviderNative, Config: models.DefaultCRMProviderConfig()}
	}
	wasHubSpot := row.Provider == provider
	if upd.Provider != nil {
		switch *upd.Provider {
		case models.CRMProviderNative, models.CRMProviderHubSpot:
			row.Provider = *upd.Provider
		default:
			return nil, errx.New(errx.BadRequest, "provider must be native or hubspot")
		}
		if !wasHubSpot && row.Provider == provider && upd.ConnectionID == nil {
			// Another CRM's connection cannot run HubSpot mode.
			row.ConnectionID = nil
		}
	}
	if upd.ConnectionID != nil {
		row.ConnectionID = upd.ConnectionID
	}
	if upd.Config != nil {
		if verr := upd.Config.Validate(); verr != nil {
			return nil, errx.New(errx.BadRequest, verr.Error())
		}
		row.Config = *upd.Config
		row.Config.For = provider
	} else if row.Provider == provider && !row.Config.BelongsTo(provider) {
		// The stored choices name another CRM's stages and properties.
		row.Config = models.DefaultCRMProviderConfigFor(provider)
		row.Config.For = provider
	}
	if row.Provider == provider {
		if row.ConnectionID == nil {
			return nil, errx.New(errx.BadRequest, "connection_id is required to use HubSpot as the CRM")
		}
		conn, cerr := s.d.Tokens.GetConnection(ctx, orgID, *row.ConnectionID)
		if cerr != nil || conn == nil || conn.Provider != models.IntegrationHubSpot {
			return nil, errx.New(errx.BadRequest, "connection_id must name a connected HubSpot account")
		}
		if acct := accountView(conn); len(acct.MissingScopes) > 0 {
			return nil, errx.NewWithIdentifier(errx.Conflict, "crm_reauth_required",
				"Reconnect HubSpot to grant the permissions CRM mode needs (deals, companies and owners).")
		}
	}
	if upd.CompleteSetup {
		now := time.Now().UTC()
		row.SetupCompletedAt = &now
	}
	if err := s.d.Repo.UpsertSettings(ctx, row); err != nil {
		return nil, errx.InternalError()
	}
	s.forget(orgID)

	if row.Provider == provider {
		if o, rerr := s.resolve(ctx, orgID); rerr == nil && o != nil {
			if perr := s.propertiesReady(ctx, o); perr != nil {
				return nil, s.userError(ctx, o, perr)
			}
			if !wasHubSpot {
				go s.initialPull(orgID)
			}
		}
	}
	return s.Settings(ctx, orgID)
}

// initialPull fills the mirror right after a switch: owners, pipelines, then
// recent deals and tasks.
func (s *Service) initialPull(orgID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	defer func() {
		if r := recover(); r != nil {
			log.Error().Interface("panic", r).Msg("hubspot: initial pull panicked")
		}
	}()
	key := "pull:" + orgID.String()
	if !s.claim(ctx, key, 10*time.Minute) {
		return
	}
	defer s.release(ctx, key)
	o, err := s.resolve(ctx, orgID)
	if err != nil || o == nil {
		return
	}
	s.pullOrg(ctx, o, true)
}

// Metadata is the HubSpot vocabulary the pickers render.
func (s *Service) Metadata(ctx context.Context, orgID uuid.UUID) (*models.CRMMetadata, *errx.Error) {
	o, xerr := s.mustResolve(ctx, orgID)
	if xerr != nil {
		return nil, xerr
	}
	props, err := s.contactProperties(ctx, o)
	if err != nil {
		return nil, s.userError(ctx, o, err)
	}
	out := &models.CRMMetadata{
		LifecycleStages: []models.CRMOption{},
		LeadStatuses:    []models.CRMOption{},
		TaskTypes:       []models.CRMOption{},
		Properties:      []models.CRMProperty{},
		Pipelines:       []models.CRMOption{},
	}
	for _, p := range props {
		switch p.Name {
		case "lifecyclestage":
			out.LifecycleStages = optionsOf(p)
		case "hs_lead_status":
			out.LeadStatuses = optionsOf(p)
		}
		if p.Hidden || p.Calculated {
			continue
		}
		out.Properties = append(out.Properties, models.CRMProperty{
			Name: p.Name, Label: p.Label, Type: p.Type, GroupName: p.GroupName, ReadOnly: p.ModificationMetadata.ReadOnlyValue,
		})
	}
	sort.Slice(out.Properties, func(i, j int) bool {
		return strings.ToLower(out.Properties[i].Label) < strings.ToLower(out.Properties[j].Label)
	})
	for _, t := range taskTypeOptions {
		out.TaskTypes = append(out.TaskTypes, models.CRMOption{Value: t.value, Label: t.label})
	}
	pipes, err := o.Client.DealPipelines(ctx)
	if err != nil {
		return nil, s.userError(ctx, o, err)
	}
	for _, p := range pipes {
		if !p.Archived {
			out.Pipelines = append(out.Pipelines, models.CRMOption{Value: p.ID, Label: p.Label})
		}
	}
	return out, nil
}

func optionsOf(p Property) []models.CRMOption {
	opts := append([]PropertyOption(nil), p.Options...)
	sort.SliceStable(opts, func(i, j int) bool { return opts[i].DisplayOrder < opts[j].DisplayOrder })
	out := make([]models.CRMOption, 0, len(opts))
	for _, op := range opts {
		if !op.Hidden {
			out = append(out, models.CRMOption{Value: op.Value, Label: op.Label})
		}
	}
	return out
}

// contactProperties is the portal's contact schema, cached for ten minutes.
func (s *Service) contactProperties(ctx context.Context, o *org) ([]Property, error) {
	key := "hubspot:props:" + o.Portal
	if s.d.Cache != nil {
		if raw, err := s.d.Cache.Get(ctx, key).Bytes(); err == nil && len(raw) > 0 {
			var props []Property
			if json.Unmarshal(raw, &props) == nil {
				return props, nil
			}
		}
	}
	props, err := o.Client.Properties(ctx, "contacts")
	if err != nil {
		return nil, err
	}
	if s.d.Cache != nil {
		_ = s.d.Cache.SetJSON(ctx, key, props, 10*time.Minute)
	}
	return props, nil
}

func (s *Service) optionLabel(ctx context.Context, o *org, property, value string) string {
	if value == "" {
		return ""
	}
	props, err := s.contactProperties(ctx, o)
	if err != nil {
		return value
	}
	for _, p := range props {
		if p.Name != property {
			continue
		}
		for _, op := range p.Options {
			if op.Value == value {
				return op.Label
			}
		}
	}
	return value
}

// Owners lists HubSpot owners with their member match.
func (s *Service) Owners(ctx context.Context, orgID uuid.UUID) ([]models.CRMOwner, *errx.Error) {
	owners, err := s.d.Repo.ListOwners(ctx, orgID, provider)
	if err != nil {
		return nil, errx.InternalError()
	}
	return owners, nil
}

// MapOwner pins (or clears) the member an owner is.
func (s *Service) MapOwner(ctx context.Context, orgID uuid.UUID, externalID string, userID *uuid.UUID) *errx.Error {
	if err := s.d.Repo.SetOwnerUser(ctx, orgID, provider, externalID, userID); err != nil {
		if err == repository.ErrCRMRecordNotFound {
			return errx.New(errx.NotFound, "owner or member not found")
		}
		return errx.InternalError()
	}
	return nil
}

// SyncHealth reports the outbox and the pulls.
func (s *Service) SyncHealth(ctx context.Context, orgID uuid.UUID) (*models.CRMSyncHealth, *errx.Error) {
	h, err := s.d.Repo.SyncHealth(ctx, orgID, provider)
	if err != nil {
		return nil, errx.InternalError()
	}
	return h, nil
}

// RetryFailed requeues failed jobs (all when ids is empty).
func (s *Service) RetryFailed(ctx context.Context, orgID uuid.UUID, ids []uuid.UUID) (int64, *errx.Error) {
	n, err := s.d.Repo.RetryFailedJobs(ctx, orgID, provider, ids)
	if err != nil {
		return 0, errx.InternalError()
	}
	return n, nil
}

// DiscardFailed drops failed jobs (all when ids is empty).
func (s *Service) DiscardFailed(ctx context.Context, orgID uuid.UUID, ids []uuid.UUID) (int64, *errx.Error) {
	n, err := s.d.Repo.DiscardFailedJobs(ctx, orgID, provider, ids)
	if err != nil {
		return 0, errx.InternalError()
	}
	return n, nil
}

// SyncNow runs a full pull for the workspace in the background.
func (s *Service) SyncNow(ctx context.Context, orgID uuid.UUID) *errx.Error {
	if _, xerr := s.mustResolve(ctx, orgID); xerr != nil {
		return xerr
	}
	if s.d.Cache != nil {
		ok, err := s.d.Cache.SetNX(ctx, "hubspot:syncnow:"+orgID.String(), 1, 30*time.Second).Result()
		if err == nil && !ok {
			return errx.NewWithIdentifier(errx.TooManyRequests, "crm_sync_running", "A sync just started. It finishes in a minute or two.")
		}
	}
	go s.initialPull(orgID)
	return nil
}
