package poollink

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/app/analytics"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

type reportMailboxRepo struct {
	repository.PoolLinkRepository
	instance uuid.UUID
	mailbox  models.PoolLinkMailbox
}

func (r reportMailboxRepo) ListReportMailboxes(_ context.Context, instance uuid.UUID, requested []uuid.UUID) ([]models.PoolLinkMailbox, error) {
	if instance != r.instance {
		return nil, nil
	}
	for _, id := range requested {
		if id == r.mailbox.RemoteID {
			return []models.PoolLinkMailbox{r.mailbox}, nil
		}
	}
	return nil, nil
}

type reportAnalytics struct {
	analytics.AnalyticsService
	org   uuid.UUID
	ids   []uuid.UUID
	calls int
}

func (a *reportAnalytics) GetWarmupStatsForAccounts(_ context.Context, org uuid.UUID, ids []uuid.UUID, _, _ time.Time) ([]models.WarmupDailyStats, *errx.Error) {
	a.org, a.ids = org, ids
	a.calls++
	return []models.WarmupDailyStats{{Date: "2026-10-01", EmailsSent: 9, Active: true}}, nil
}

func (a *reportAnalytics) GetWarmupPlacementDataForAccounts(_ context.Context, org uuid.UUID, ids []uuid.UUID, _, _ time.Time) (*models.WarmupPlacementData, *errx.Error) {
	a.org, a.ids = org, ids
	a.calls++
	return &models.WarmupPlacementData{
		Daily:       []models.WarmupPlacementDayRow{{SenderID: ids[0], Inbox: 9}},
		Sent:        []models.WarmupSenderDayCount{{SenderID: ids[0], Count: 10}},
		Unconfirmed: []models.WarmupSenderDayCount{{SenderID: ids[0], Count: 1}},
		Windows:     map[uuid.UUID]models.WarmupPlacementWindow{ids[0]: {Major: models.WarmupPlacementTally{Inbox: 9}}},
	}, nil
}

func TestReportsScopeAndMapTheCallingInstancesMailboxes(t *testing.T) {
	inst := &models.PoolLinkInstance{ID: uuid.New(), OrganizationID: uuid.New()}
	mailbox := models.PoolLinkMailbox{RemoteID: uuid.New(), EmailAccountID: uuid.New()}
	a := &reportAnalytics{}
	s := &service{repo: reportMailboxRepo{instance: inst.ID, mailbox: mailbox}, analytics: a}
	req := models.PoolLinkWarmupReportRequest{RemoteIDs: []uuid.UUID{mailbox.RemoteID, mailbox.RemoteID}, From: "2026-10-01", To: "2026-10-02"}
	stats, xerr := s.WarmupStats(context.Background(), inst, req)
	if xerr != nil || len(stats) != 1 || !stats[0].Active || a.org != inst.OrganizationID || len(a.ids) != 1 || a.ids[0] != mailbox.EmailAccountID {
		t.Fatalf("stats=%+v error=%v scope=%v/%v", stats, xerr, a.org, a.ids)
	}
	data, xerr := s.WarmupPlacementData(context.Background(), inst, req)
	if xerr != nil || data.Daily[0].SenderID != mailbox.RemoteID || data.Sent[0].SenderID != mailbox.RemoteID || data.Unconfirmed[0].SenderID != mailbox.RemoteID || data.Windows[mailbox.RemoteID].Major.Inbox != 9 {
		t.Fatalf("data=%+v error=%v", data, xerr)
	}
	calls := a.calls
	for _, ids := range [][]uuid.UUID{{uuid.New()}, {mailbox.RemoteID, uuid.New()}, {mailbox.EmailAccountID}} {
		req.RemoteIDs = ids
		if _, xerr := s.WarmupStats(context.Background(), inst, req); xerr != ErrMailboxNotFound || a.calls != calls {
			t.Fatalf("unauthorized IDs reached analytics: %v, calls=%d", xerr, a.calls)
		}
		if _, xerr := s.WarmupPlacementData(context.Background(), inst, req); xerr != ErrMailboxNotFound || a.calls != calls {
			t.Fatalf("unauthorized placement IDs reached analytics: %v", xerr)
		}
	}
	req.RemoteIDs = []uuid.UUID{mailbox.RemoteID}
	if _, xerr := s.WarmupStats(context.Background(), &models.PoolLinkInstance{ID: uuid.New()}, req); xerr != ErrMailboxNotFound {
		t.Fatalf("another instance accessed the report: %v", xerr)
	}
}
