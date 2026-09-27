package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

type periodCampaignRepoStub struct {
	repository.CampaignRepository
}

func (periodCampaignRepoStub) Get(context.Context, string, string) (*models.Campaign, error) {
	return &models.Campaign{ID: uuid.New()}, nil
}

// periodAnalyticsRepoStub records the period every campaign read was given.
type periodAnalyticsRepoStub struct {
	repository.AnalyticsRepository
	seen []*models.DateRange
}

func (s *periodAnalyticsRepoStub) GetCampaignSummary(_ context.Context, _, _ uuid.UUID, p *models.DateRange) (*models.CampaignSummary, *errx.Error) {
	s.seen = append(s.seen, p)
	return &models.CampaignSummary{}, nil
}

func (s *periodAnalyticsRepoStub) GetSequenceStats(_ context.Context, _ uuid.UUID, p *models.DateRange) ([]models.SequenceStats, *errx.Error) {
	s.seen = append(s.seen, p)
	return nil, nil
}

func (s *periodAnalyticsRepoStub) GetCampaignEngagementBreakdown(_ context.Context, _ uuid.UUID, p *models.DateRange, _ int) (*models.CampaignEngagementBreakdown, *errx.Error) {
	s.seen = append(s.seen, p)
	return nil, nil
}

// Issue #702: summary, steps and engagement all read the one period asked for.
func TestCampaignAnalyticsReadsOnePeriod(t *testing.T) {
	asked := &models.DateRange{From: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)}
	for _, period := range []*models.DateRange{nil, asked} {
		repo := &periodAnalyticsRepoStub{}
		svc := &analyticsService{analyticsRepo: repo, campaignRepo: periodCampaignRepoStub{}}
		got, xerr := svc.GetCampaignAnalytics(context.Background(), uuid.New(), uuid.New(), period)
		if xerr != nil {
			t.Fatalf("GetCampaignAnalytics: %v", xerr)
		}
		if len(repo.seen) != 3 {
			t.Fatalf("%d reads, want summary, steps and engagement", len(repo.seen))
		}
		for i, p := range repo.seen {
			if p != period {
				t.Errorf("read %d got period %+v, want %+v", i, p, period)
			}
		}
		if period != nil && got.DateRange != *period {
			t.Errorf("date_range = %+v, want the period asked for", got.DateRange)
		}
	}
}

// date_range names the window the figures cover instead of a zero value.
func TestCampaignPeriodResolvesAllTime(t *testing.T) {
	now := time.Date(2026, 9, 27, 0, 30, 0, 0, time.FixedZone("PDT", -7*3600)) // Sep 27 07:30 UTC
	today := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	campaign := &models.Campaign{CreatedAt: time.Date(2026, 5, 3, 17, 0, 0, 0, time.FixedZone("PDT", -7*3600))}
	firstSend := time.Date(2026, 5, 9, 23, 30, 0, 0, time.UTC)
	asked := &models.DateRange{From: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)}

	for _, tc := range []struct {
		name     string
		period   *models.DateRange
		first    *time.Time
		wantFrom time.Time
		wantTo   time.Time
	}{
		{"asked period", asked, &firstSend, asked.From, asked.To},
		{"all time starts at the first send's UTC day", nil, &firstSend, time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC), today},
		{"all time before any send starts at creation's UTC day", nil, nil, time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC), today},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := campaignPeriod(campaign, tc.first, tc.period, now)
			if !got.From.Equal(tc.wantFrom) || !got.To.Equal(tc.wantTo) {
				t.Errorf("date_range = %s..%s, want %s..%s", got.From, got.To, tc.wantFrom, tc.wantTo)
			}
		})
	}
}
