package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/models"
)

func singleWarmupAccount(id *uuid.UUID) []uuid.UUID {
	if id == nil {
		return nil
	}
	return []uuid.UUID{*id}
}

func (r *warmupPlacementRepository) ForAccounts(ctx context.Context, orgID uuid.UUID, ids []uuid.UUID, from, to time.Time) (*models.WarmupPlacementData, error) {
	d := &models.WarmupPlacementData{}
	var err error
	lookback := from.AddDate(0, 0, -(models.WarmupPlacementWindowDays - 1))
	if d.Daily, err = r.daily(ctx, orgID, ids, lookback, to); err != nil {
		return nil, err
	}
	if d.Hosts, err = r.hosts(ctx, orgID, ids, from, to); err != nil {
		return nil, err
	}
	if d.Sent, err = r.sent(ctx, orgID, ids, from, to); err != nil {
		return nil, err
	}
	if d.Unconfirmed, err = r.unconfirmed(ctx, orgID, ids, from, to, time.Now().Add(-models.WarmupUnconfirmedAfterHours*time.Hour)); err != nil {
		return nil, err
	}
	since := time.Now().UTC().AddDate(0, 0, -(models.WarmupPlacementWindowDays - 1))
	since = time.Date(since.Year(), since.Month(), since.Day(), 0, 0, 0, 0, time.UTC)
	if d.Windows, err = r.rates(ctx, orgID, ids, since); err != nil {
		return nil, err
	}
	return d, nil
}
