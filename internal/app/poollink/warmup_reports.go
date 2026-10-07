package poollink

import (
	"context"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

func (s *service) reportAccounts(ctx context.Context, inst *models.PoolLinkInstance, remoteIDs []uuid.UUID) ([]uuid.UUID, map[uuid.UUID]uuid.UUID, *errx.Error) {
	enrolled, err := s.repo.ListReportMailboxes(ctx, inst.ID, remoteIDs)
	if err != nil {
		return nil, nil, errx.InternalError()
	}
	byRemote := make(map[uuid.UUID]uuid.UUID, len(enrolled))
	for _, mailbox := range enrolled {
		byRemote[mailbox.RemoteID] = mailbox.EmailAccountID
	}
	ids := make([]uuid.UUID, 0, len(remoteIDs))
	remotes := make(map[uuid.UUID]uuid.UUID, len(remoteIDs))
	for _, remote := range remoteIDs {
		id, ok := byRemote[remote]
		if !ok {
			return nil, nil, ErrMailboxNotFound
		}
		if _, seen := remotes[id]; !seen {
			ids = append(ids, id)
			remotes[id] = remote
		}
	}
	return ids, remotes, nil
}

func (s *service) WarmupStats(ctx context.Context, inst *models.PoolLinkInstance, req models.PoolLinkWarmupReportRequest) ([]models.WarmupDailyStats, *errx.Error) {
	from, to, err := req.Range()
	if err != nil {
		return nil, errx.New(errx.BadRequest, err.Error())
	}
	ids, _, xerr := s.reportAccounts(ctx, inst, req.RemoteIDs)
	if xerr != nil {
		return nil, xerr
	}
	return s.analytics.GetWarmupStatsForAccounts(ctx, inst.OrganizationID, ids, from, to)
}

func (s *service) WarmupPlacementData(ctx context.Context, inst *models.PoolLinkInstance, req models.PoolLinkWarmupReportRequest) (*models.WarmupPlacementData, *errx.Error) {
	from, to, err := req.Range()
	if err != nil {
		return nil, errx.New(errx.BadRequest, err.Error())
	}
	ids, remotes, xerr := s.reportAccounts(ctx, inst, req.RemoteIDs)
	if xerr != nil {
		return nil, xerr
	}
	data, xerr := s.analytics.GetWarmupPlacementDataForAccounts(ctx, inst.OrganizationID, ids, from, to)
	if xerr != nil {
		return nil, xerr
	}
	if err := data.RemapSenders(remotes); err != nil {
		return nil, errx.InternalError()
	}
	return data, nil
}
