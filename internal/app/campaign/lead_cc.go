package campaign

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/config"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

// leadCCSuggestionLimit bounds the colleagues offered when picking copies.
const leadCCSuggestionLimit = 8

func (s *campaignService) ListLeadCC(ctx context.Context, orgID, campaignID, contactID uuid.UUID) ([]models.CampaignLeadCC, *errx.Error) {
	if s.campaignProgressRepo == nil {
		return nil, errx.InternalError()
	}
	if xerr := s.ownedCampaign(ctx, orgID, campaignID); xerr != nil {
		return nil, xerr
	}
	if _, err := s.campaignProgressRepo.GetLeadHold(ctx, campaignID, contactID); err != nil {
		if errors.Is(err, repository.ErrLeadNotInCampaign) {
			return nil, errx.New(errx.NotFound, "contact is not a lead of this campaign")
		}
		return nil, errx.InternalError()
	}
	cc, err := s.campaignProgressRepo.ListLeadCC(ctx, campaignID, contactID)
	if err != nil {
		return nil, errx.InternalError()
	}
	return cc, nil
}

func (s *campaignService) SetLeadCC(ctx context.Context, orgID, campaignID, contactID uuid.UUID, contactIDs []string) ([]models.CampaignLeadCC, *errx.Error) {
	if s.campaignProgressRepo == nil {
		return nil, errx.InternalError()
	}
	if xerr := s.ownedCampaign(ctx, orgID, campaignID); xerr != nil {
		return nil, xerr
	}
	ids := make([]uuid.UUID, 0, len(contactIDs))
	seen := map[uuid.UUID]bool{}
	for _, raw := range contactIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			return nil, errx.New(errx.BadRequest, "contact_ids must be contact ids")
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) > config.CampaignLeadMaxCC {
		return nil, errx.NewWithIdentifier(errx.BadRequest, "lead_cc_limit",
			fmt.Sprintf("A lead can have at most %d contacts copied on their emails", config.CampaignLeadMaxCC))
	}

	before, err := s.campaignProgressRepo.ListLeadCC(ctx, campaignID, contactID)
	if err != nil {
		return nil, errx.InternalError()
	}
	switch err := s.campaignProgressRepo.SetLeadCC(ctx, orgID, campaignID, contactID, ids); {
	case err == nil:
	case errors.Is(err, repository.ErrLeadNotInCampaign):
		return nil, errx.New(errx.NotFound, "contact is not a lead of this campaign")
	case errors.Is(err, repository.ErrLeadCCSelf):
		return nil, errx.NewWithIdentifier(errx.BadRequest, "lead_cc_self", "A lead cannot be copied on their own emails")
	case errors.Is(err, repository.ErrLeadCCContactNotFound):
		return nil, errx.NewWithIdentifier(errx.NotFound, "lead_cc_contact_not_found", "A contact to copy was not found in this workspace")
	case errors.Is(err, repository.ErrLeadCCLeadIsCopied):
		return nil, errx.NewWithIdentifier(errx.Conflict, "lead_cc_lead_is_copied",
			"This lead is copied on another lead's emails in this campaign, so it sends none of its own to copy anyone on")
	case errors.Is(err, repository.ErrLeadCCHasCopies):
		return nil, errx.NewWithIdentifier(errx.Conflict, "lead_cc_has_copies",
			"A contact to copy has contacts copied on their own emails in this campaign; remove those first")
	default:
		return nil, errx.InternalError()
	}

	// A removed copy's own lead is released and may be due now.
	for _, b := range before {
		if !seen[b.ContactID] {
			s.WakeCampaigns(ctx, orgID, []string{campaignID.String()})
			break
		}
	}

	cc, err := s.campaignProgressRepo.ListLeadCC(ctx, campaignID, contactID)
	if err != nil {
		return nil, errx.InternalError()
	}
	return cc, nil
}

func (s *campaignService) SuggestLeadCC(ctx context.Context, orgID, campaignID, contactID uuid.UUID) ([]models.CampaignLeadCCSuggestion, *errx.Error) {
	if s.campaignProgressRepo == nil {
		return nil, errx.InternalError()
	}
	if xerr := s.ownedCampaign(ctx, orgID, campaignID); xerr != nil {
		return nil, xerr
	}
	out, err := s.campaignProgressRepo.SuggestLeadCC(ctx, orgID, campaignID, contactID, leadCCSuggestionLimit)
	if err != nil {
		return nil, errx.InternalError()
	}
	return out, nil
}
