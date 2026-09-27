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
	campaign models.Campaign
}

func (s periodCampaignRepoStub) Get(context.Context, string, string) (*models.Campaign, error) {
	c := s.campaign
	return &c, nil
}

// periodAnalyticsRepoStub records the period every campaign read was given.
type periodAnalyticsRepoStub struct {
	repository.AnalyticsRepository
	first *time.Time
	seen  []*models.DateRange
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

func (s *periodAnalyticsRepoStub) GetCampaignFirstSentAt(context.Context, uuid.UUID) (*time.Time, *errx.Error) {
	return s.first, nil
}

// Issue #702: every figure reads the same period, and date_range names the
// window they cover instead of a zero value.
func TestCampaignAnalyticsReportsThePeriodItRead(t *testing.T) {
	today := utcDay(time.Now())
	created := time.Date(2026, 5, 3, 17, 0, 0, 0, time.FixedZone("PDT", -7*3600))
	firstSend := time.Date(2026, 5, 9, 23, 30, 0, 0, time.UTC)
	asked := &models.DateRange{From: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)}

	for _, tc := range []struct {
		name     string
		period   *models.DateRange
		first    *time.Time
		wantFrom time.Time
		wantTo   time.Time
	}{
		{"asked period", asked, &firstSend, asked.From, asked.To},
		{"all time starts at the first send", nil, &firstSend, time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC), today},
		{"all time before any send starts at creation", nil, nil, time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC), today},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &periodAnalyticsRepoStub{first: tc.first}
			svc := &analyticsService{
				analyticsRepo: repo,
				campaignRepo:  periodCampaignRepoStub{campaign: models.Campaign{ID: uuid.New(), CreatedAt: created}},
			}
			got, xerr := svc.GetCampaignAnalytics(context.Background(), uuid.New(), uuid.New(), tc.period)
			if xerr != nil {
				t.Fatalf("GetCampaignAnalytics: %v", xerr)
			}
			if len(repo.seen) != 3 {
				t.Fatalf("%d reads, want summary, steps and engagement", len(repo.seen))
			}
			for i, p := range repo.seen {
				if p != tc.period {
					t.Errorf("read %d got period %+v, want %+v", i, p, tc.period)
				}
			}
			if !got.DateRange.From.Equal(tc.wantFrom) || !got.DateRange.To.Equal(tc.wantTo) {
				t.Errorf("date_range = %s..%s, want %s..%s", got.DateRange.From, got.DateRange.To, tc.wantFrom, tc.wantTo)
			}
		})
	}
}
