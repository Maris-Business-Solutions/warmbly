package campaign

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/app/segment"
	"github.com/warmbly/warmbly/internal/infrastructure/db"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

// A saved draft's estimate counts its own leads together with the segments,
// once each, and only for a campaign in the caller's workspace.
func TestLiveEstimateCountsASavedDraftsLeads(t *testing.T) {
	dsn := os.Getenv("WARMBLY_TEST_DB")
	if dsn == "" {
		t.Skip("WARMBLY_TEST_DB not set")
	}
	ctx := context.Background()
	handle, err := db.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(handle.Pool.Close)
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := handle.Exec(ctx, q, args...); err != nil {
			t.Fatalf("%s: %v", q[:min(60, len(q))], err)
		}
	}

	user, org, otherOrg := uuid.New(), uuid.New(), uuid.New()
	draft, foreign, seg, linked := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	a, b, c, d, e := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	exec(`INSERT INTO users (id, email, first_name, last_name) VALUES ($1, $2, 'Estimate', 'Test')`, user, user.String()+"@test.local")
	t.Cleanup(func() {
		exec(`DELETE FROM campaigns WHERE organization_id = ANY($1)`, []uuid.UUID{org, otherOrg})
		exec(`DELETE FROM segments WHERE organization_id = ANY($1)`, []uuid.UUID{org, otherOrg})
		exec(`DELETE FROM contacts WHERE organization_id = ANY($1)`, []uuid.UUID{org, otherOrg})
		exec(`DELETE FROM organizations WHERE id = ANY($1)`, []uuid.UUID{org, otherOrg})
		exec(`DELETE FROM users WHERE id = $1`, user)
	})
	for _, id := range []uuid.UUID{org, otherOrg} {
		exec(`INSERT INTO organizations (id, name, slug, owner_user_id) VALUES ($1, 'Estimate', $2, $3)`, id, id.String(), user)
	}
	exec(`INSERT INTO campaigns (id, user_id, organization_id, name, description, days, updated_at, created_at) VALUES ($1, $2, $3, 'Draft', '', 31, NOW(), NOW()), ($4, $2, $5, 'Foreign', '', 31, NOW(), NOW())`, draft, user, org, foreign, otherOrg)
	for _, id := range []uuid.UUID{a, b, c, d, e} {
		exec(`INSERT INTO contacts (id, user_id, organization_id, email, first_name, last_name, company, phone, custom_fields) VALUES ($1, $2, $3, $4, 'L', 'Ead', '', '', '{}')`, id, user, org, id.String()+"@test.local")
	}
	// a and b are leads already; a and c are on the chosen segment. d came in
	// through the linked segment and is still on it, so unlinking takes d away;
	// e came in the same way but has left it, so e stays. Four people in all.
	exec(`INSERT INTO campaign_leads (campaign_id, contact_id) VALUES ($1, $2), ($1, $3)`, draft, a, b)
	exec(`INSERT INTO campaign_leads (campaign_id, contact_id, source) VALUES ($1, $2, 'segment'), ($1, $3, 'segment')`, draft, d, e)
	exec(`INSERT INTO segments (id, organization_id, created_by, name) VALUES ($1, $2, $3, 'List'), ($4, $2, $3, 'Linked')`, seg, org, user, linked)
	exec(`INSERT INTO segment_members (segment_id, contact_id, mode) VALUES ($1, $2, 'include'), ($1, $3, 'include'), ($4, $5, 'include')`, seg, a, c, linked, d)
	exec(`INSERT INTO campaign_segments (campaign_id, segment_id) VALUES ($1, $2)`, draft, linked)

	svc := NewService(repository.NewCampaignRepostory(handle), repository.NewTaskRepository(handle.Pool), repository.NewEmailRepostory(handle, nil), nil, nil, nil, nil, nil, nil).(*campaignService)
	svc.WireSegments(segment.NewService(repository.NewSegmentRepository(handle), nil))

	id := draft.String()
	out, xerr := svc.Estimate(ctx, org, &models.CampaignEstimate{SegmentIDs: []string{seg.String()}, CampaignID: &id})
	if xerr != nil {
		t.Fatal(xerr)
	}
	if out.Recipients != 4 {
		t.Fatalf("recipients %d, want a, b, c and the lead that left its segment", out.Recipients)
	}
	out, xerr = svc.Estimate(ctx, org, &models.CampaignEstimate{SegmentIDs: []string{}, CampaignID: &id})
	if xerr != nil || out.Recipients != 3 {
		t.Fatalf("leads alone: %v %v, want a, b and e", out, xerr)
	}
	kept := []string{seg.String(), linked.String()}
	out, xerr = svc.Estimate(ctx, org, &models.CampaignEstimate{SegmentIDs: kept, CampaignID: &id})
	if xerr != nil || out.Recipients != 5 {
		t.Fatalf("keeping the link: %v %v, want everyone", out, xerr)
	}

	// A window saved back to front falls back rather than failing the estimate.
	exec(`UPDATE campaigns SET start_time = '19:00', end_time = '18:00' WHERE id = $1`, draft)
	if _, xerr := svc.Estimate(ctx, org, &models.CampaignEstimate{SegmentIDs: []string{}, CampaignID: &id}); xerr != nil {
		t.Fatalf("a saved backwards window failed the estimate: %v", xerr)
	}

	other := foreign.String()
	if _, xerr := svc.Estimate(ctx, org, &models.CampaignEstimate{SegmentIDs: []string{}, CampaignID: &other}); xerr == nil {
		t.Fatal("a campaign in another workspace was estimated")
	}
	late, early := "18:00", "08:00"
	if _, xerr := svc.Estimate(ctx, org, &models.CampaignEstimate{SegmentIDs: []string{}, StartTime: &late, EndTime: &early}); xerr == nil {
		t.Fatal("a window that ends before it starts was accepted")
	}
}
