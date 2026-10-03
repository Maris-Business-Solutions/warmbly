package analytics

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

type warmupAnalyticsRepoStub struct {
	repository.AnalyticsRepository
	stats []models.WarmupDailyStats
}

func (s warmupAnalyticsRepoStub) GetWarmupStats(context.Context, uuid.UUID, *uuid.UUID, time.Time, time.Time) ([]models.WarmupDailyStats, *errx.Error) {
	return s.stats, nil
}

type accountStatusEmailRepoStub struct {
	repository.EmailRepository
	emailID uuid.UUID
}

func (s accountStatusEmailRepoStub) Get(context.Context, string, string) (*models.Email, *errx.Error) {
	return &models.Email{ID: s.emailID}, nil
}

func (s accountStatusEmailRepoStub) Search(context.Context, string, string, *string, *string, int32, []uuid.UUID) (*models.EmailsResult, *errx.Error) {
	return &models.EmailsResult{Data: []models.Email{{ID: s.emailID}}}, nil
}

type accountStatusAnalyticsRepoStub struct {
	repository.AnalyticsRepository
}

func (accountStatusAnalyticsRepoStub) GetAccountDailyUsage(context.Context, uuid.UUID, time.Time) (*models.AccountDailyUsage, *errx.Error) {
	return nil, errx.InternalError()
}

func (accountStatusAnalyticsRepoStub) GetAccountDailyUsageBatch(context.Context, []uuid.UUID, time.Time) (map[uuid.UUID]*models.AccountDailyUsage, *errx.Error) {
	return nil, errx.InternalError()
}

type dashboardAnalyticsRepoStub struct {
	repository.AnalyticsRepository
	overallFrom time.Time
	overallTo   time.Time
	trendFrom   time.Time
	trendTo     time.Time
}

func (s *dashboardAnalyticsRepoStub) GetDashboardOverallStats(_ context.Context, _ uuid.UUID, from, to time.Time) (*models.DashboardOverallStats, *errx.Error) {
	s.overallFrom, s.overallTo = from, to
	return &models.DashboardOverallStats{}, nil
}

func (*dashboardAnalyticsRepoStub) GetRecentActivity(context.Context, uuid.UUID, int) ([]models.RecentActivityItem, *errx.Error) {
	return []models.RecentActivityItem{}, nil
}

func (*dashboardAnalyticsRepoStub) GetTopCampaigns(context.Context, uuid.UUID, time.Time, time.Time, int, string) ([]models.TopCampaignStats, *errx.Error) {
	return []models.TopCampaignStats{}, nil
}

func (*dashboardAnalyticsRepoStub) GetAccountHealthSummary(context.Context, uuid.UUID) (*models.AccountHealthSummary, *errx.Error) {
	return &models.AccountHealthSummary{}, nil
}

func (s *dashboardAnalyticsRepoStub) GetDashboardDailyTrend(_ context.Context, _ uuid.UUID, from, to time.Time) ([]models.DashboardDailyStats, *errx.Error) {
	s.trendFrom, s.trendTo = from, to
	return []models.DashboardDailyStats{}, nil
}

func TestWarmupSummaryReportsEveryDisplayedMetric(t *testing.T) {
	repo := warmupAnalyticsRepoStub{stats: []models.WarmupDailyStats{
		{Date: "2026-09-14", EmailsSent: 5, EmailsReplied: 2, EmailsReceived: 4, TargetVolume: 10, Active: true},
		{Date: "2026-09-15", EmailsSent: 10, EmailsReplied: 3, EmailsReceived: 7, TargetVolume: 10, Active: true},
		// A day the mailbox was written to but had no plan: it lists, and is
		// not a day active.
		{Date: "2026-09-16", EmailsReceived: 2},
	}}
	svc := &analyticsService{analyticsRepo: repo}

	got, xerr := svc.GetWarmupAnalytics(context.Background(), uuid.New(), nil, time.Time{}, time.Now())
	if xerr != nil {
		t.Fatalf("GetWarmupAnalytics: %v", xerr)
	}
	if got.Summary.TotalSent != 15 || got.Summary.TotalReplied != 5 || got.Summary.DaysActive != 2 {
		t.Errorf("summary totals = sent %d, replied %d, days %d; want 15/5/2",
			got.Summary.TotalSent, got.Summary.TotalReplied, got.Summary.DaysActive)
	}
	if got.Summary.TotalReceived != 13 {
		t.Errorf("total_received = %d, want 13", got.Summary.TotalReceived)
	}
	if math.Abs(got.Summary.AverageDaily-7.5) > 0.001 {
		t.Errorf("average_daily = %.3f, want 7.5", got.Summary.AverageDaily)
	}
	if math.Abs(got.Summary.ReplyRate-33.333) > 0.01 {
		t.Errorf("reply_rate = %.3f, want 33.333", got.Summary.ReplyRate)
	}
	if math.Abs(got.Summary.TargetProgress-75) > 0.001 {
		t.Errorf("target_progress = %.3f, want 75", got.Summary.TargetProgress)
	}
}

func TestAccountStatusDoesNotTurnUsageErrorsIntoZero(t *testing.T) {
	emailID := uuid.New()
	svc := &analyticsService{
		analyticsRepo: accountStatusAnalyticsRepoStub{},
		emailRepo:     accountStatusEmailRepoStub{emailID: emailID},
	}

	status, xerr := svc.GetAccountStatus(context.Background(), uuid.New(), uuid.New())
	if xerr == nil || status != nil {
		t.Fatalf("GetAccountStatus = (%+v, %v), want an error and no plausible zero usage", status, xerr)
	}
}

func TestAccountStatusListDoesNotHideUsageErrors(t *testing.T) {
	emailID := uuid.New()
	svc := &analyticsService{
		analyticsRepo: accountStatusAnalyticsRepoStub{},
		emailRepo:     accountStatusEmailRepoStub{emailID: emailID},
	}

	statuses, xerr := svc.GetAllAccountStatuses(context.Background(), uuid.New())
	if xerr == nil || statuses != nil {
		t.Fatalf("GetAllAccountStatuses = (%+v, %v), want an error instead of an incomplete list", statuses, xerr)
	}
}

func TestDashboardUsesTheSameSevenCalendarDaysForCardsAndTrend(t *testing.T) {
	repo := &dashboardAnalyticsRepoStub{}
	svc := &analyticsService{analyticsRepo: repo}

	got, xerr := svc.GetDashboardAnalytics(context.Background(), uuid.New(), "7d")
	if xerr != nil {
		t.Fatalf("GetDashboardAnalytics: %v", xerr)
	}
	if got.Period != "7d" {
		t.Fatalf("period = %q, want 7d", got.Period)
	}
	if !repo.overallFrom.Equal(repo.trendFrom) || !repo.overallTo.Equal(repo.trendTo) {
		t.Fatalf("card range %s..%s differs from trend range %s..%s", repo.overallFrom, repo.overallTo, repo.trendFrom, repo.trendTo)
	}
	if repo.overallFrom.Location() != time.UTC || repo.overallFrom.Hour() != 0 || repo.overallFrom.Minute() != 0 {
		t.Fatalf("range starts at %s, want UTC midnight", repo.overallFrom)
	}
	if days := int(repo.overallTo.Sub(repo.overallFrom).Hours() / 24); days != 6 {
		t.Fatalf("range starts %d whole days before today, want 6", days)
	}
}
