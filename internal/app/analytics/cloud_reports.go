package analytics

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

type CloudWarmupReports interface {
	WarmupStats(ctx context.Context, orgID uuid.UUID, id *uuid.UUID, from, to time.Time) ([]models.WarmupDailyStats, *errx.Error)
	WarmupPlacementData(ctx context.Context, orgID uuid.UUID, id *uuid.UUID, from, to time.Time) (*models.WarmupPlacementData, *errx.Error)
}

type CloudWarmupReportsAware interface {
	WireCloudWarmupReports(CloudWarmupReports)
}

func (s *analyticsService) WireCloudWarmupReports(reports CloudWarmupReports) {
	s.cloudReports = reports
}

func (s *analyticsService) GetWarmupStatsForAccounts(ctx context.Context, orgID uuid.UUID, ids []uuid.UUID, from, to time.Time) ([]models.WarmupDailyStats, *errx.Error) {
	return s.analyticsRepo.GetWarmupStatsForAccounts(ctx, orgID, ids, from, to)
}

func (s *analyticsService) GetWarmupPlacementDataForAccounts(ctx context.Context, orgID uuid.UUID, ids []uuid.UUID, from, to time.Time) (*models.WarmupPlacementData, *errx.Error) {
	if s.placementRepo == nil {
		return nil, errx.InternalError()
	}
	data, err := s.placementRepo.ForAccounts(ctx, orgID, ids, from, to)
	if err != nil {
		return nil, errx.InternalError()
	}
	return data, nil
}
