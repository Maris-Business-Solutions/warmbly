package pipedrive

import (
	"context"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/warmbly/warmbly/internal/app/integration"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

// Settings returns the workspace's CRM mode with the connected account.
func (s *Service) Settings(ctx context.Context, orgID uuid.UUID) (*models.CRMSettings, *errx.Error) {
	row, err := s.d.Repo.GetSettings(ctx, orgID)
	if err != nil {
		return nil, errx.InternalError()
	}
	out := &models.CRMSettings{OrganizationID: orgID, Provider: models.CRMProviderNative, Config: models.DefaultCRMProviderConfigFor(provider)}
	if row != nil {
		out.Provider = row.Provider
		out.ConnectionID = row.ConnectionID
		out.Config = row.Config
		out.SetupCompletedAt = row.SetupCompletedAt
		out.UpdatedAt = row.UpdatedAt
		if !row.Config.BelongsTo(provider) && row.Provider != models.CRMProviderHubSpot {
			out.Config = models.DefaultCRMProviderConfigFor(provider)
		}
	}
	if out.ConnectionID != nil {
		if conn, cerr := s.d.Tokens.GetConnection(ctx, orgID, *out.ConnectionID); cerr == nil && conn != nil {
			out.Account = accountView(conn)
		}
	}
	if out.Provider == provider && out.Account == nil {
		// The Pipedrive connection was removed: the CRM is Warmbly's again.
		out.Provider = models.CRMProviderNative
	}
	return out, nil
}

func accountView(conn *models.IntegrationConnection) *models.CRMAccount {
	acct := &models.CRMAccount{
		ExternalID: firstNonEmpty(displayString(conn.DisplayFields, "company_id"), conn.ExternalAccountID),
		Name:       conn.ExternalAccountName,
		Status:     string(conn.Status),
		Health:     conn.Health,
	}
	domain := displayString(conn.DisplayFields, "company_domain")
	if domain == "" {
		if base, err := integration.PipedriveAPIDomain(displayString(conn.DisplayFields, "api_domain")); err == nil {
			domain = domainOf(base)
		}
	}
	if domain != "" {
		acct.AppURL = "https://" + domain + ".pipedrive.com"
	}
	if len(conn.GrantedScopes) > 0 && !slices.Contains(conn.GrantedScopes, "admin") {
		for _, sc := range integration.PipedriveRequiredScopes {
			if !slices.Contains(conn.GrantedScopes, sc) {
				acct.MissingScopes = append(acct.MissingScopes, sc)
			}
		}
	}
	return acct
}

// UpdateSettings switches the workspace's CRM and stores its choices. Choosing
// Pipedrive seeds the rules from the company's own labels, registers the
// webhooks and starts the first pull, so the dashboard has data by the time
// the wizard closes.
func (s *Service) UpdateSettings(ctx context.Context, orgID uuid.UUID, upd *models.UpdateCRMSettings) (*models.CRMSettings, *errx.Error) {
	row, err := s.d.Repo.GetSettings(ctx, orgID)
	if err != nil {
		return nil, errx.InternalError()
	}
	if row == nil {
		row = &repository.CRMSettingsRow{OrganizationID: orgID, Provider: models.CRMProviderNative, Config: models.DefaultCRMProviderConfigFor(provider)}
	}
	wasPipedrive := row.Provider == provider
	if upd.Provider != nil {
		switch *upd.Provider {
		case models.CRMProviderNative, models.CRMProviderPipedrive:
			row.Provider = *upd.Provider
		default:
			return nil, errx.New(errx.BadRequest, "provider must be native or pipedrive")
		}
		if !wasPipedrive && row.Provider == provider && upd.ConnectionID == nil {
			row.ConnectionID = nil
		}
	}
	if upd.ConnectionID != nil {
		row.ConnectionID = upd.ConnectionID
	}
	seed := false
	if upd.Config != nil {
		if verr := upd.Config.Validate(); verr != nil {
			return nil, errx.New(errx.BadRequest, verr.Error())
		}
		row.Config = *upd.Config
		row.Config.For = provider
	} else if row.Provider == provider && !row.Config.BelongsTo(provider) {
		row.Config = models.DefaultCRMProviderConfigFor(provider)
		row.Config.For = provider
		seed = true
	}
	var conn *models.IntegrationConnection
	if row.Provider == provider {
		if row.ConnectionID == nil {
			return nil, errx.New(errx.BadRequest, "connection_id is required to use Pipedrive as the CRM")
		}
		c, cerr := s.d.Tokens.GetConnection(ctx, orgID, *row.ConnectionID)
		if cerr != nil || c == nil || c.Provider != models.IntegrationPipedrive {
			return nil, errx.New(errx.BadRequest, "connection_id must name a connected Pipedrive account")
		}
		if acct := accountView(c); len(acct.MissingScopes) > 0 {
			return nil, errx.NewWithIdentifier(errx.Conflict, "crm_reauth_required",
				"Reconnect Pipedrive to grant the permissions CRM mode needs (deals, people, activities and users).")
		}
		conn = c
	}
	if upd.CompleteSetup {
		now := time.Now().UTC()
		row.SetupCompletedAt = &now
	}
	if conn != nil && !wasPipedrive {
		// A token Pipedrive refuses is found before the switch, not after it.
		o := s.orgFor(ctx, orgID, conn, row)
		if _, err := o.Client.Me(ctx); err != nil {
			if xe := s.userError(ctx, o, err); xe.Identifier == "crm_reauth_required" {
				s.Forget(orgID)
				return nil, xe
			}
		}
		if seed {
			s.seedRules(ctx, o, &row.Config)
		}
	}
	if err := s.d.Repo.UpsertSettings(ctx, row); err != nil {
		return nil, errx.InternalError()
	}
	s.Forget(orgID)

	if row.Provider == provider && !wasPipedrive {
		go s.initialPull(orgID)
	}
	return s.Settings(ctx, orgID)
}

// seedRules fills the label rules from the company's own labels: an
// interested reply labels the person a hot lead, and a customer is never
// emailed by a campaign.
func (s *Service) seedRules(ctx context.Context, o *org, cfg *models.CRMProviderConfig) {
	labels := s.labels(ctx, o)
	if hot := labelByName(labels, "Hot lead", "Hot", "Warm lead"); hot != "" {
		cfg.PositiveReply.LifecycleStage = hot
	}
	if customer := labelByName(labels, "Customer", "Client"); customer != "" {
		cfg.ExitRules.LifecycleStages = []string{customer}
		cfg.Guards.SkipLifecycleStages = []string{customer}
	}
}

// initialPull fills the mirror right after a switch: users, pipelines,
// webhooks, then recent deals and activities.
func (s *Service) initialPull(orgID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	defer func() {
		if r := recover(); r != nil {
			log.Error().Interface("panic", r).Msg("pipedrive: initial pull panicked")
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

// Metadata is the Pipedrive vocabulary the pickers render. Person labels take
// the place of lifecycle stages; Pipedrive has no lead status.
func (s *Service) Metadata(ctx context.Context, orgID uuid.UUID) (*models.CRMMetadata, *errx.Error) {
	o, xerr := s.mustResolve(ctx, orgID)
	if xerr != nil {
		return nil, xerr
	}
	fields, err := s.personFields(ctx, o)
	if err != nil {
		return nil, s.userError(ctx, o, err)
	}
	out := &models.CRMMetadata{
		LifecycleStages: s.labels(ctx, o),
		LeadStatuses:    []models.CRMOption{},
		TaskTypes:       []models.CRMOption{},
		Properties:      append([]models.CRMProperty{}, standardPersonKeys...),
		Pipelines:       []models.CRMOption{},
	}
	if out.LifecycleStages == nil {
		out.LifecycleStages = []models.CRMOption{}
	}
	var custom []models.CRMProperty
	for _, f := range fields {
		if !f.IsCustomField || f.FieldCode == "" {
			continue
		}
		custom = append(custom, models.CRMProperty{
			Name: f.FieldCode, Label: f.FieldName, Type: f.FieldType, GroupName: "Custom fields",
			ReadOnly: !mappableTypes[f.FieldType] || strings.HasPrefix(strings.ToLower(f.FieldName), "warmbly "),
		})
	}
	sort.Slice(custom, func(i, j int) bool { return strings.ToLower(custom[i].Label) < strings.ToLower(custom[j].Label) })
	out.Properties = append(out.Properties, custom...)
	for _, t := range s.activityTypes(ctx, o) {
		out.TaskTypes = append(out.TaskTypes, models.CRMOption{Value: t.KeyString, Label: t.Name})
	}
	pipes, err := o.Client.Pipelines(ctx)
	if err != nil {
		return nil, s.userError(ctx, o, err)
	}
	slices.SortStableFunc(pipes, func(a, b Pipeline) int { return a.OrderNr - b.OrderNr })
	for _, p := range pipes {
		if !p.IsDeleted {
			out.Pipelines = append(out.Pipelines, models.CRMOption{Value: id(p.ID), Label: p.Name})
		}
	}
	return out, nil
}

// Owners lists Pipedrive users with their member match.
func (s *Service) Owners(ctx context.Context, orgID uuid.UUID) ([]models.CRMOwner, *errx.Error) {
	owners, err := s.d.Repo.ListOwners(ctx, orgID, provider)
	if err != nil {
		return nil, errx.InternalError()
	}
	return owners, nil
}

// MapOwner pins (or clears) the member a Pipedrive user is.
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
		ok, err := s.d.Cache.SetNX(ctx, "pipedrive:syncnow:"+orgID.String(), 1, time.Minute).Result()
		if err == nil && !ok {
			return errx.NewWithIdentifier(errx.TooManyRequests, "crm_sync_running", "A sync just started. It finishes in a minute or two.")
		}
	}
	go s.initialPull(orgID)
	return nil
}
