package scheduler

import (
	"context"
	"testing"

	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

// The projection reads the pool through the real repositories and has to
// account for every email it promises: the timeline adds up to the audience
// times its steps, and the mailbox never exceeds its own cap on any day.
func TestLiveProjectCampaignAddsUp(t *testing.T) {
	handle, pool := liveDB(t)
	f := newLiveFixture(t, pool, "UTC")
	planner, ok := loggedScheduler(t, f).(CampaignSendPlanner)
	if !ok {
		t.Fatal("the scheduler service does not project")
	}
	ctx := context.Background()
	accounts, xerr := repository.NewEmailRepostory(handle, testEncrypter(t)).GetAllActiveInScope(ctx, repository.NewAccountScope(&f.org))
	if xerr != nil {
		t.Fatal(xerr)
	}
	if len(accounts) != 1 {
		t.Fatalf("got %d mailboxes, want the fixture's one", len(accounts))
	}

	out, err := planner.ProjectCampaign(ctx, CampaignProjectionInput{
		Campaign:      &models.Campaign{OrganizationID: &f.org, DailyLimit: 50, Days: 0b0011111, StartTime: "08:00", EndTime: "18:00", Timezone: "UTC"},
		Accounts:      accounts,
		Recipients:    120,
		StepWaits:     []int{3},
		OrgDailyLimit: -1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Steps != 2 || out.TotalSends != 240 || out.Mailboxes != 1 || len(out.Senders) != 1 {
		t.Fatalf("steps %d total %d mailboxes %d senders %d", out.Steps, out.TotalSends, out.Mailboxes, len(out.Senders))
	}
	if out.EstimatedFinishAt == nil || out.FirstTouchFinishAt == nil || out.FirstTouchFinishAt.After(*out.EstimatedFinishAt) {
		t.Fatalf("finish %v first touch %v", out.EstimatedFinishAt, out.FirstTouchFinishAt)
	}
	if out.SteadyCapacity <= 0 || out.SteadyCapacity > 50 {
		t.Fatalf("steady capacity %d, want within the mailbox cap", out.SteadyCapacity)
	}
	total := 0
	for _, d := range out.Timeline {
		if d.Sends > d.Capacity {
			t.Fatalf("%s sends %d over capacity %d", d.Date, d.Sends, d.Capacity)
		}
		if !d.SendingDay && d.Sends > 0 {
			t.Fatalf("%s is not a sending day but sends %d", d.Date, d.Sends)
		}
		total += d.Sends
	}
	if total != out.TotalSends {
		t.Fatalf("timeline sends %d, want %d", total, out.TotalSends)
	}
}
