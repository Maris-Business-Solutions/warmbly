package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/models"
)

// Issue #702: a campaign's performance can be read for a period. The period is
// a send cohort: the emails sent on its days, with every open, click, reply and
// bounce they earned whenever it came, and nothing earned by an older send.
//
//	WARMBLY_TEST_DB=postgres://warmbly:warmbly@localhost:15432/<db>?sslmode=disable \
//	  go test ./internal/repository/ -run LiveCampaignAnalyticsPeriod -v
func TestLiveCampaignAnalyticsPeriodIsASendCohort(t *testing.T) {
	handle, pool := liveContactDB(t)
	f := newSharedOrgFixture(t, pool)
	ctx := context.Background()

	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("seed %q: %v", sql[:min(60, len(sql))], err)
		}
	}
	intro, follow := uuid.New(), uuid.New()
	for i, step := range []uuid.UUID{intro, follow} {
		exec(`INSERT INTO sequences (id, campaign_id, organization_id, name, subject, body_plain, body_html, wait_after, position, kind)
		      VALUES ($1, $2, $3, $4, 'Hi', 'Hello', '<p>Hello</p>', 0, $5, 'email')`,
			step, f.campaign, f.org, []string{"Intro", "Follow-up"}[i], i+1)
	}

	day := func(d, h, m int) time.Time { return time.Date(2026, time.March, d, h, m, 0, 0, time.UTC) }
	period := &models.DateRange{From: day(1, 0, 0), To: day(7, 0, 0)}

	type send struct {
		step                              uuid.UUID
		sent                              time.Time
		opened, clicked, replied, bounced *time.Time
		openCountry                       string
		openAt                            time.Time
	}
	at := func(t time.Time) *time.Time { return &t }
	sends := map[string]send{
		// Sent the second before the period; its reply lands inside it.
		"before": {step: intro, sent: day(1, 0, 0).Add(-time.Second), opened: at(day(2, 9, 0)), replied: at(day(2, 10, 0)), openCountry: "FR", openAt: day(2, 9, 0)},
		// First instant of the period; the reply comes three days after it.
		"first": {step: intro, sent: day(1, 0, 0), opened: at(day(1, 8, 0)), replied: at(day(10, 12, 0)), openCountry: "DE", openAt: day(1, 8, 0)},
		// Inside; opened and clicked after the period ended.
		"middle": {step: intro, sent: day(4, 12, 0), opened: at(day(12, 9, 0)), clicked: at(day(12, 9, 5)), openCountry: "US", openAt: day(12, 9, 0)},
		// Late on the last day, which a `<= to` at midnight would drop.
		"last": {step: follow, sent: day(7, 23, 30), bounced: at(day(7, 23, 31))},
		// The first instant after the period.
		"after": {step: follow, sent: day(8, 0, 0), opened: at(day(8, 1, 0)), openCountry: "JP", openAt: day(8, 1, 0)},
	}
	for tag, s := range sends {
		contact := addLead(t, f, "i702-"+tag+"-"+uuid.New().String()[:6]+"@test.local", "valid", true)
		exec(`INSERT INTO campaign_contact_progress (campaign_id, contact_id, sequence_id, sent_at, opened_at, clicked_at, replied_at, bounced_at)
		      VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			f.campaign, contact, s.step, s.sent, s.opened, s.clicked, s.replied, s.bounced)
		if s.openCountry != "" {
			exec(`INSERT INTO email_opens (id, task_id, campaign_id, contact_id, sequence_id, opened_at, machine, machine_reason, country_code)
			      VALUES ($1, $2, $3, $4, $5, $6, false, '', $7)`,
				uuid.New(), uuid.New(), f.campaign, contact, s.step, s.openAt, s.openCountry)
		}
		if s.clicked != nil {
			exec(`INSERT INTO email_link_clicks (task_id, campaign_id, contact_id, sequence_id, destination, clicked_at, country_code, machine, machine_reason)
			      VALUES ($1, $2, $3, $4, 'https://example.com', $5, $6, false, '')`,
				uuid.New(), f.campaign, contact, s.step, *s.clicked, s.openCountry)
		}
	}
	// A lead nothing was sent to yet: campaign state, not performance.
	addLead(t, f, "i702-queued-"+uuid.New().String()[:6]+"@test.local", "valid", true)

	repo := &analyticsRepository{DB: handle}

	all, xerr := repo.GetCampaignSummary(ctx, f.org, f.campaign, nil)
	if xerr != nil {
		t.Fatalf("all-time summary: %v", xerr)
	}
	sum, xerr := repo.GetCampaignSummary(ctx, f.org, f.campaign, period)
	if xerr != nil {
		t.Fatalf("period summary: %v", xerr)
	}
	if all.EmailsSent != 5 {
		t.Errorf("all-time sent = %d, want every send (5)", all.EmailsSent)
	}
	// first, middle and last; the reply to "before" and the open on "after"
	// are not the period's, and first's late reply and middle's late click are.
	if sum.EmailsSent != 3 || sum.UniqueOpens != 2 || sum.UniqueClicks != 1 || sum.Replies != 1 || sum.Bounces != 1 {
		t.Errorf("period summary = sent %d opens %d clicks %d replies %d bounces %d, want 3/2/1/1/1",
			sum.EmailsSent, sum.UniqueOpens, sum.UniqueClicks, sum.Replies, sum.Bounces)
	}
	if want := models.Rate(1, 3); sum.ReplyRate < want-0.01 || sum.ReplyRate > want+0.01 {
		t.Errorf("period reply rate = %.2f, want %.2f of the period's own sends", sum.ReplyRate, want)
	}
	if sum.TotalContacts != all.TotalContacts || sum.EmailsPending != all.EmailsPending {
		t.Errorf("contacts/pending = %d/%d in the period and %d/%d all time, want the campaign's state either way",
			sum.TotalContacts, sum.EmailsPending, all.TotalContacts, all.EmailsPending)
	}
	// Six leads on two steps, five sent.
	if all.TotalContacts != 6 || all.EmailsPending != 7 {
		t.Errorf("contacts/pending = %d/%d, want 6/7", all.TotalContacts, all.EmailsPending)
	}

	steps, xerr := repo.GetSequenceStats(ctx, f.campaign, period)
	if xerr != nil {
		t.Fatalf("period steps: %v", xerr)
	}
	if len(steps) != 2 {
		t.Fatalf("got %d steps, want 2", len(steps))
	}
	if s := steps[0]; s.EmailsSent != 2 || s.Replies != 1 || s.Clicks != 1 {
		t.Errorf("intro in period = sent %d replies %d clicks %d, want 2/1/1", s.EmailsSent, s.Replies, s.Clicks)
	}
	if s := steps[1]; s.EmailsSent != 1 || s.Bounces != 1 || s.Opens != 0 {
		t.Errorf("follow-up in period = sent %d bounces %d opens %d, want 1/1/0", s.EmailsSent, s.Bounces, s.Opens)
	}
	var stepSent int
	for _, s := range steps {
		stepSent += s.EmailsSent
	}
	if stepSent != sum.EmailsSent {
		t.Errorf("steps add up to %d sent, the summary says %d", stepSent, sum.EmailsSent)
	}

	daily, xerr := repo.GetCampaignDailyStats(ctx, f.campaign, period.From, period.To)
	if xerr != nil {
		t.Fatalf("daily: %v", xerr)
	}
	var dailySent int
	for _, d := range daily {
		dailySent += d.Sent
	}
	if dailySent != sum.EmailsSent || len(daily) == 0 || daily[len(daily)-1].Date != "2026-03-07" {
		t.Errorf("daily = %+v, want %d sends ending on 2026-03-07", daily, sum.EmailsSent)
	}

	cmp, xerr := repo.CompareCampaigns(ctx, f.org, []uuid.UUID{f.campaign}, period.From, period.To)
	if xerr != nil {
		t.Fatalf("compare: %v", xerr)
	}
	if len(cmp.Campaigns) != 1 || cmp.Campaigns[0].EmailsSent != 3 {
		t.Errorf("compare = %+v, want the same 3 sends, the last day's included", cmp.Campaigns)
	}

	eng, xerr := repo.GetCampaignEngagementBreakdown(ctx, f.campaign, period, 8)
	if xerr != nil {
		t.Fatalf("period engagement: %v", xerr)
	}
	countries := map[string]models.EngagementBucket{}
	for _, b := range eng.Countries {
		countries[b.Key] = b
	}
	if len(countries) != 2 || countries["DE"].Opens != 1 || countries["US"].Opens != 1 || countries["US"].Clicks != 1 {
		t.Errorf("period countries = %+v, want DE and US only (FR's send predates the period, JP's follows it)", eng.Countries)
	}
	allEng, xerr := repo.GetCampaignEngagementBreakdown(ctx, f.campaign, nil, 8)
	if xerr != nil {
		t.Fatalf("all-time engagement: %v", xerr)
	}
	if len(allEng.Countries) != 4 {
		t.Errorf("all-time countries = %+v, want all four", allEng.Countries)
	}

	first, xerr := repo.GetCampaignFirstSentAt(ctx, f.campaign)
	if xerr != nil {
		t.Fatalf("first sent: %v", xerr)
	}
	if first == nil || !first.Equal(sends["before"].sent) {
		t.Errorf("first sent = %v, want %v", first, sends["before"].sent)
	}
	if none, xerr := repo.GetCampaignFirstSentAt(ctx, f.other); xerr != nil || none != nil {
		t.Errorf("a campaign that never sent: first = %v, err %v, want nil", none, xerr)
	}
}
