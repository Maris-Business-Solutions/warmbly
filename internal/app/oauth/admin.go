package oauth

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
	"github.com/warmbly/warmbly/internal/utils/paging"
)

const maxModerationReason = 1000

// WireAdmin attaches the operator's moderation repository.
func (s *Service) WireAdmin(r repository.OAuthAdminRepository) {
	s.admin = r
}

func (s *Service) adminReady() *errx.Error {
	if s.admin == nil {
		return errx.New(errx.ServiceUnavailable, "app moderation is not available")
	}
	return nil
}

func moderationReason(reason string, required bool) (string, *errx.Error) {
	reason = strings.TrimSpace(reason)
	if required && reason == "" {
		return "", errx.New(errx.BadRequest, "a reason is required; the developer sees it")
	}
	if utf8.RuneCountInString(reason) > maxModerationReason {
		return "", errx.New(errx.BadRequest, "the reason is too long")
	}
	return reason, nil
}

func (s *Service) AdminListApps(ctx context.Context, q *models.AdminOAuthAppSearch) (*models.AdminOAuthAppsResult, *errx.Error) {
	if xerr := s.adminReady(); xerr != nil {
		return nil, xerr
	}
	if q.Status != "" && q.Status != "active" && q.Status != "disabled" && q.Status != "suspended" {
		return nil, errx.New(errx.BadRequest, "invalid status filter")
	}
	if q.Limit <= 0 || q.Limit > 200 {
		q.Limit = 50
	}
	rows, total, err := s.admin.ListApps(ctx, q)
	if err != nil {
		return nil, errx.New(errx.Internal, "lookup failed")
	}
	res := &models.AdminOAuthAppsResult{Data: rows, Pagination: models.Pagination{Total: &total}}
	if int64(q.Offset+len(rows)) < total {
		res.Pagination.HasMore = true
		res.Pagination.NextCursor = paging.EncodeOffset(q.Offset + len(rows))
	}
	return res, nil
}

func (s *Service) adminApp(ctx context.Context, id uuid.UUID) (*models.AdminOAuthApp, *errx.Error) {
	a, err := s.admin.GetApp(ctx, id)
	if err != nil {
		return nil, errx.New(errx.Internal, "lookup failed")
	}
	if a == nil {
		return nil, errx.New(errx.NotFound, "application not found")
	}
	return a, nil
}

// AdminSuspend stops the app everywhere: no consent, no token exchange, no
// accepted tokens, no webhook deliveries, out of the directory.
func (s *Service) AdminSuspend(ctx context.Context, id, adminID uuid.UUID, reason string) (*models.AdminOAuthApp, *errx.Error) {
	if xerr := s.adminReady(); xerr != nil {
		return nil, xerr
	}
	reason, xerr := moderationReason(reason, true)
	if xerr != nil {
		return nil, xerr
	}
	if _, xerr := s.adminApp(ctx, id); xerr != nil {
		return nil, xerr
	}
	if _, err := s.admin.Suspend(ctx, id, adminID, reason); err != nil {
		return nil, errx.New(errx.Internal, "suspend failed")
	}
	s.ReconcileAppEndpoints(ctx, id)
	return s.adminApp(ctx, id)
}

func (s *Service) AdminUnsuspend(ctx context.Context, id uuid.UUID) (*models.AdminOAuthApp, *errx.Error) {
	if xerr := s.adminReady(); xerr != nil {
		return nil, xerr
	}
	if _, xerr := s.adminApp(ctx, id); xerr != nil {
		return nil, xerr
	}
	if err := s.admin.Unsuspend(ctx, id); err != nil {
		return nil, errx.New(errx.Internal, "unsuspend failed")
	}
	s.ReconcileAppEndpoints(ctx, id)
	return s.adminApp(ctx, id)
}

// AdminRevokeGrants ends every token the app holds; each workspace has to
// authorize it again.
func (s *Service) AdminRevokeGrants(ctx context.Context, id uuid.UUID) (int64, *errx.Error) {
	if xerr := s.adminReady(); xerr != nil {
		return 0, xerr
	}
	if _, xerr := s.adminApp(ctx, id); xerr != nil {
		return 0, xerr
	}
	n, err := s.admin.RevokeAllGrants(ctx, id)
	if err != nil {
		return 0, errx.New(errx.Internal, "revoke failed")
	}
	s.ReconcileAppEndpoints(ctx, id)
	return n, nil
}

// AdminRemoveLogo clears a logo that should not be shown to anyone and
// returns the address it had, so the caller can delete the stored image.
func (s *Service) AdminRemoveLogo(ctx context.Context, id uuid.UUID) (*models.AdminOAuthApp, string, *errx.Error) {
	if xerr := s.adminReady(); xerr != nil {
		return nil, "", xerr
	}
	before, xerr := s.adminApp(ctx, id)
	if xerr != nil {
		return nil, "", xerr
	}
	if err := s.admin.ClearLogo(ctx, id); err != nil {
		return nil, "", errx.New(errx.Internal, "update failed")
	}
	after, xerr := s.adminApp(ctx, id)
	return after, before.LogoURL, xerr
}

func (s *Service) AdminListBlocks(ctx context.Context) ([]models.OAuthDeveloperBlock, *errx.Error) {
	if xerr := s.adminReady(); xerr != nil {
		return nil, xerr
	}
	blocks, err := s.admin.ListBlocks(ctx)
	if err != nil {
		return nil, errx.New(errx.Internal, "lookup failed")
	}
	return blocks, nil
}

// AdminBlock stops a workspace or a person from registering and publishing
// apps, and optionally suspends the apps they already have.
func (s *Service) AdminBlock(ctx context.Context, adminID uuid.UUID, req models.CreateOAuthDeveloperBlock) (*models.OAuthDeveloperBlock, int, *errx.Error) {
	if xerr := s.adminReady(); xerr != nil {
		return nil, 0, xerr
	}
	if (req.OrganizationID == nil) == (req.UserID == nil) {
		return nil, 0, errx.New(errx.BadRequest, "block either a workspace or a person")
	}
	reason, xerr := moderationReason(req.Reason, true)
	if xerr != nil {
		return nil, 0, xerr
	}
	b, err := s.admin.CreateBlock(ctx, req.OrganizationID, req.UserID, adminID, reason)
	if errors.Is(err, repository.ErrDeveloperBlockExists) {
		return nil, 0, errx.NewWithIdentifier(errx.Conflict, "developer_block_exists", "already blocked")
	}
	if err != nil {
		return nil, 0, errx.New(errx.Internal, "block failed")
	}
	suspended := 0
	if req.SuspendApps {
		var ids []uuid.UUID
		if req.OrganizationID != nil {
			ids, err = s.admin.SuspendOrgApps(ctx, *req.OrganizationID, adminID, reason)
		} else {
			ids, err = s.admin.SuspendCreatorApps(ctx, *req.UserID, adminID, reason)
		}
		if err != nil {
			return b, 0, errx.New(errx.Internal, "blocked, but suspending the existing apps failed")
		}
		for _, id := range ids {
			s.ReconcileAppEndpoints(ctx, id)
		}
		suspended = len(ids)
	}
	return b, suspended, nil
}

// AdminUnblock lifts a block. Apps suspended with it stay suspended until
// unsuspended one by one, so lifting a block is never a blanket reinstatement.
func (s *Service) AdminUnblock(ctx context.Context, id uuid.UUID) *errx.Error {
	if xerr := s.adminReady(); xerr != nil {
		return xerr
	}
	ok, err := s.admin.DeleteBlock(ctx, id)
	if err != nil {
		return errx.New(errx.Internal, "unblock failed")
	}
	if !ok {
		return errx.New(errx.NotFound, "block not found")
	}
	return nil
}
