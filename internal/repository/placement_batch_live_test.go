package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/models"
)

// Run against a migrated scratch database:
//
//	WARMBLY_TEST_DB=postgres://warmbly:warmbly@localhost:15432/<db>?sslmode=disable \
//	  go test ./internal/repository/ -run LivePlacementBatch -v

func TestLivePlacementBatchLifecycle(t *testing.T) {
	f := newPlacementFixture(t)
	handle, pool := liveContactDB(t)
	requireSchemaVersion(t, pool, 230)
	batches := NewPlacementBatchRepository(handle)
	ctx := context.Background()
	now := time.Now()

	// A second sender on another domain, and a tag on the first.
	other := uuid.New()
	f.exec(`INSERT INTO email_accounts (id, user_id, organization_id, email, name, signature_plain, signature_html, provider, campaign_limit)
	        VALUES ($1, $2, $3, $4, 'Placement', '', '', 'outlook', 50)`, other, f.owner, f.org, "rep-"+other.String()[:8]+"@beta.test")
	tagID := uuid.New()
	f.exec(`INSERT INTO tags (id, organization_id, user_id, title, color, position) VALUES ($1, $2, $3, 'Client A', '#0ea5e9', 0)`, tagID, f.org, f.owner)
	f.exec(`INSERT INTO email_tags (email_id, tag_id) VALUES ($1, $2)`, f.sender, tagID)
	f.exec(`UPDATE email_accounts SET status = 'active' WHERE organization_id = $1`, f.org)

	all, err := batches.ListBatchCandidates(ctx, f.org, PlacementCandidateFilter{})
	if err != nil || len(all) != 2 {
		t.Fatalf("ListBatchCandidates = %d, %v; want both senders", len(all), err)
	}
	tagged, err := batches.ListBatchCandidates(ctx, f.org, PlacementCandidateFilter{TagIDs: []uuid.UUID{tagID}})
	if err != nil || len(tagged) != 1 || tagged[0].ID != f.sender {
		t.Fatalf("tag filter = %+v, %v; want the tagged sender", tagged, err)
	}
	// Another workspace's mailboxes (the operator's seeds) never appear.
	if foreign, err := batches.ListBatchCandidates(ctx, f.org, PlacementCandidateFilter{IDs: f.seeds}); err != nil || len(foreign) != 0 {
		t.Fatalf("foreign ids resolved %d, %v", len(foreign), err)
	}

	b := &models.PlacementBatch{
		ID: uuid.New(), OrganizationID: f.org, Subject: "Quick question", BodyPlain: "Hi there",
		Tracking: models.PlacementTrackingCompare, Panel: models.PlacementPanelInstance, Pace: models.PlacementPaceSpaced,
		OnUnavailable: models.PlacementUnavailableDefer, SenderCount: 2, Status: models.PlacementBatchQueued,
		Selection:  models.PlacementBatchSelection{Scope: &models.PlacementSenderScope{Type: models.PlacementScopeWorkspace}, Matched: 2},
		RetryUntil: now.Add(7 * 24 * time.Hour),
	}
	senderRow, otherRow := uuid.New(), uuid.New()
	sender, otherID := f.sender, other
	if err := batches.CreateBatch(ctx, b, []models.PlacementBatchSender{
		{ID: senderRow, EmailAccountID: &sender, SenderEmail: "a@acme.test", SenderDomain: "acme.test", SenderFamily: "gmail", Position: 0},
		{ID: otherRow, EmailAccountID: &otherID, SenderEmail: "b@beta.test", SenderDomain: "beta.test", SenderFamily: "outlook", Position: 1},
	}); err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}
	if n, err := batches.CountOpenBatches(ctx, f.org); err != nil || n != 1 {
		t.Fatalf("CountOpenBatches = %d, %v", n, err)
	}

	claimed, err := batches.ClaimBatches(ctx, now, 2*time.Minute, 100)
	if err != nil {
		t.Fatalf("ClaimBatches: %v", err)
	}
	var mine *models.PlacementBatch
	for i := range claimed {
		if claimed[i].ID == b.ID {
			mine = &claimed[i]
		}
	}
	if mine == nil || mine.LastTickAt != nil || mine.Selection.Matched != 2 {
		t.Fatalf("claimed batch = %+v; want it with no previous tick and its selection", mine)
	}
	again, err := batches.ClaimBatches(ctx, now, 2*time.Minute, 100)
	if err != nil {
		t.Fatalf("ClaimBatches: %v", err)
	}
	for _, c := range again {
		if c.ID == b.ID {
			t.Fatalf("a leased batch was claimed twice")
		}
	}
	if err := batches.ReleaseBatch(ctx, b.ID); err != nil {
		t.Fatalf("ReleaseBatch: %v", err)
	}
	again, err = batches.ClaimBatches(ctx, now.Add(time.Second), time.Minute, 100)
	if err != nil || !containsBatch(again, b.ID) {
		t.Fatalf("a released batch was not claimable (%v)", err)
	}
	for _, c := range again {
		if c.ID == b.ID && (c.LastTickAt == nil || c.LastTickAt.Sub(now).Abs() > time.Millisecond) {
			t.Fatalf("second claim reports last tick %v; want the first claim's %v", c.LastTickAt, now)
		}
	}
	_ = batches.ReleaseBatch(ctx, b.ID)

	due, err := batches.DueBatchSenders(ctx, b.ID, now.Add(time.Minute), 10)
	if err != nil || len(due) != 2 || due[0].ID != senderRow {
		t.Fatalf("DueBatchSenders = %+v, %v; want both in position order", due, err)
	}
	if ok, err := batches.ClaimBatchSender(ctx, senderRow); err != nil || !ok {
		t.Fatalf("ClaimBatchSender = %v, %v", ok, err)
	}
	if ok, _ := batches.ClaimBatchSender(ctx, senderRow); ok {
		t.Fatalf("a running sender was claimed twice")
	}
	// Claimed with no test yet: it counts as sending.
	if n, err := batches.CountSendingBatchSenders(ctx, &f.org); err != nil || n != 1 {
		t.Fatalf("CountSendingBatchSenders = %d, %v; want the claimed sender", n, err)
	}

	// The claimed sender gets its two tests (a comparison); one copy lands in
	// the inbox and one in spam for the tracked half, one inbox untracked.
	batchID, rowID := b.ID, senderRow
	mk := func(tracked bool, folders ...string) models.PlacementTest {
		test := models.PlacementTest{
			ID: uuid.New(), OrganizationID: &f.org, SenderAccountID: &sender, SenderEmail: "a@acme.test",
			Subject: "Quick question", BodyPlain: "Hi there", Origin: models.PlacementOriginBatch,
			Panel: models.PlacementPanelInstance, Status: models.PlacementStatusRunning,
			OpenTracking: tracked, BatchID: &batchID, BatchSenderID: &rowID,
		}
		var results []models.PlacementResult
		for i, folder := range folders {
			seed := f.seeds[i]
			results = append(results, models.PlacementResult{
				ID: uuid.New(), SeedAccountID: &seed, SeedAddress: seed.String() + "@gmail.com", Family: "gmail", Folder: folder,
			})
		}
		if err := f.repo.CreateTest(ctx, &test, results, nil); err != nil {
			t.Fatalf("CreateTest: %v", err)
		}
		return test
	}
	tracked := mk(true, "inbox", "spam")
	untracked := mk(false, "inbox", "inbox")

	// Workspace listings leave batch tests to their batch.
	listed, _, err := f.repo.ListTests(ctx, PlacementTestFilter{OrganizationID: &f.org})
	if err != nil || len(listed) != 0 {
		t.Fatalf("ListTests = %d, %v; want batch tests left out", len(listed), err)
	}
	listed, _, err = f.repo.ListTests(ctx, PlacementTestFilter{OrganizationID: &f.org, BatchID: &batchID})
	if err != nil || len(listed) != 2 {
		t.Fatalf("ListTests by batch = %d, %v; want its two tests", len(listed), err)
	}
	if got, _ := f.repo.GetTest(ctx, tracked.ID); got == nil || got.BatchID == nil || *got.BatchID != batchID {
		t.Fatalf("GetTest lost the batch id: %+v", got)
	}

	// Nothing unsent any more: the sender stops counting as sending, and once
	// both tests finish it resolves to completed.
	if n, _ := batches.CountSendingBatchSenders(ctx, &f.org); n != 0 {
		t.Fatalf("CountSendingBatchSenders = %d with every copy resolved", n)
	}
	if err := batches.SyncBatchSenders(ctx, b.ID, now.Add(-time.Hour)); err != nil {
		t.Fatalf("SyncBatchSenders: %v", err)
	}
	if p := progressOf(t, batches, b.ID); p.Running != 1 {
		t.Fatalf("progress %+v; want the sender still running while its tests are open", p)
	}
	f.exec(`UPDATE placement_tests SET status = 'completed', finished_at = NOW() WHERE batch_id = $1`, b.ID)
	if err := batches.SyncBatchSenders(ctx, b.ID, now.Add(-time.Hour)); err != nil {
		t.Fatalf("SyncBatchSenders: %v", err)
	}
	if p := progressOf(t, batches, b.ID); p.Completed != 1 || p.Queued != 1 {
		t.Fatalf("progress %+v; want one completed and one queued", p)
	}

	// Headline is the tracked half of the comparison.
	sums, err := batches.BatchSummaries(ctx, []uuid.UUID{b.ID})
	if err != nil || sums[b.ID].Inbox != 1 || sums[b.ID].Spam != 1 {
		t.Fatalf("BatchSummaries = %+v, %v; want the tracked half only", sums[b.ID], err)
	}
	rows, err := batches.BatchBreakdown(ctx, b.ID)
	if err != nil {
		t.Fatalf("BatchBreakdown: %v", err)
	}
	sets := map[string]int{}
	var untrackedInbox int
	for _, r := range rows {
		sets[r.Set] += r.Count
		if r.Set == "overall" && !r.Tracked && r.Folder == "inbox" {
			untrackedInbox = r.Count
		}
		if r.Set == "domain" && r.SenderDomain != "acme.test" {
			t.Fatalf("domain row for %q", r.SenderDomain)
		}
		if r.Set == "matrix" && (r.SenderDomain != "acme.test" || r.RecipientFamily != "gmail") {
			t.Fatalf("matrix row %+v", r)
		}
	}
	if sets["overall"] != 4 || sets["domain"] != 2 || sets["provider"] != 2 || sets["recipient"] != 2 || sets["matrix"] != 2 || untrackedInbox != 2 {
		t.Fatalf("breakdown totals %v (untracked inbox %d); want 4 overall and the 2 tracked copies in every group", sets, untrackedInbox)
	}
	sizes, err := batches.BatchGroupSizes(ctx, b.ID)
	if err != nil {
		t.Fatalf("BatchGroupSizes: %v", err)
	}
	var domainRows, familyRows int
	for _, s := range sizes {
		if s.ByDomain {
			domainRows++
		} else {
			familyRows++
		}
		if s.ByDomain && s.Domain == "acme.test" && (s.Senders != 1 || s.Completed != 1) {
			t.Fatalf("acme.test size %+v", s)
		}
	}
	if domainRows != 2 || familyRows != 2 {
		t.Fatalf("group sizes %+v; want two domains and two providers", sizes)
	}

	senders, total, err := batches.ListBatchSenders(ctx, f.org, b.ID, PlacementBatchSenderFilter{Limit: 10})
	if err != nil || total != 2 || len(senders) != 2 {
		t.Fatalf("ListBatchSenders = %d/%d, %v", len(senders), total, err)
	}
	if senders[0].ID != senderRow || senders[0].Counts.Inbox != 1 || senders[0].Counts.Spam != 1 || len(senders[0].TestIDs) != 2 {
		t.Fatalf("first sender %+v; want the tested one with its tracked counts and both tests", senders[0])
	}
	if senders[0].TestIDs[0] != untracked.ID {
		t.Fatalf("test ids %v; want the untracked half first", senders[0].TestIDs)
	}
	found, total, err := batches.ListBatchSenders(ctx, f.org, b.ID, PlacementBatchSenderFilter{Search: "beta", Limit: 10})
	if err != nil || total != 1 || found[0].ID != otherRow {
		t.Fatalf("search = %+v, %d, %v", found, total, err)
	}
	if _, total, _ := batches.ListBatchSenders(ctx, uuid.New(), b.ID, PlacementBatchSenderFilter{Limit: 10}); total != 0 {
		t.Fatalf("another workspace listed %d of this batch's senders", total)
	}

	cov, err := batches.Coverage(ctx, f.org, time.Now())
	if err != nil || cov.Mailboxes != 2 || cov.Tested7d != 1 || cov.NeverTested != 1 {
		t.Fatalf("Coverage = %+v, %v; want one of two tested this week", cov, err)
	}
	if untested, err := batches.ListBatchCandidates(ctx, f.org, PlacementCandidateFilter{UntestedSince: ptrTime(now.Add(-24 * time.Hour))}); err != nil || len(untested) != 1 || untested[0].ID != other {
		t.Fatalf("untested filter = %+v, %v", untested, err)
	}

	// Cancelling closes the queued sender; a second cancel is refused.
	ok, running, err := batches.CancelBatch(ctx, f.org, b.ID)
	if err != nil || !ok || len(running) != 0 {
		t.Fatalf("CancelBatch = %v, %v, %v", ok, running, err)
	}
	if p := progressOf(t, batches, b.ID); p.Cancelled != 1 || p.Open() != 0 {
		t.Fatalf("after cancel %+v", p)
	}
	if ok, _, _ := batches.CancelBatch(ctx, f.org, b.ID); ok {
		t.Fatalf("a cancelled batch cancelled again")
	}
	if ok, err := batches.FinishBatch(ctx, b.ID, models.PlacementBatchCompleted, ""); err != nil || ok {
		t.Fatalf("FinishBatch on a cancelled batch = %v, %v", ok, err)
	}
}

func TestLivePlacementBatchImportedLandsClosed(t *testing.T) {
	f := newPlacementFixture(t)
	handle, pool := liveContactDB(t)
	requireSchemaVersion(t, pool, 230)
	batches := NewPlacementBatchRepository(handle)
	ctx := context.Background()

	// An imported batch is written without its active flag.
	id := uuid.New()
	f.exec(`INSERT INTO placement_batches (id, organization_id, tracking, panel, pace, sender_count, status, retry_until)
	        VALUES ($1, $2, 'campaign', 'instance', 'spaced', 1, 'running', NOW())`, id, f.org)
	f.exec(`INSERT INTO placement_batch_senders (batch_id, sender_email, position) VALUES ($1, 'a@acme.test', 0)`, id)
	if claimed, err := batches.ClaimBatches(ctx, time.Now(), time.Minute, 100); err != nil || containsBatch(claimed, id) {
		t.Fatalf("an inactive batch was claimed (%v)", err)
	}
	if err := batches.CloseInactiveBatches(ctx); err != nil {
		t.Fatalf("CloseInactiveBatches: %v", err)
	}
	got, err := batches.GetBatch(ctx, f.org, id)
	if err != nil || got == nil || got.Status != models.PlacementBatchCancelled {
		t.Fatalf("imported batch = %+v, %v; want it closed", got, err)
	}
	if p := progressOf(t, batches, id); p.Cancelled != 1 {
		t.Fatalf("imported batch senders %+v; want them cancelled", p)
	}
}

func progressOf(t *testing.T, r PlacementBatchRepository, id uuid.UUID) models.PlacementBatchProgress {
	t.Helper()
	p, err := r.BatchProgress(context.Background(), []uuid.UUID{id})
	if err != nil {
		t.Fatalf("BatchProgress: %v", err)
	}
	return p[id]
}

func containsBatch(bs []models.PlacementBatch, id uuid.UUID) bool {
	for _, b := range bs {
		if b.ID == id {
			return true
		}
	}
	return false
}

func ptrTime(t time.Time) *time.Time { return &t }

func TestLivePlacementBatchCancelResolvesRunningSenders(t *testing.T) {
	f := newPlacementFixture(t)
	handle, pool := liveContactDB(t)
	requireSchemaVersion(t, pool, 230)
	batches := NewPlacementBatchRepository(handle)
	ctx := context.Background()

	f.exec(`UPDATE email_accounts SET status = 'active' WHERE organization_id = $1`, f.org)
	b := &models.PlacementBatch{
		ID: uuid.New(), OrganizationID: f.org, Subject: "Quick question", BodyPlain: "Hi there",
		Tracking: models.PlacementTrackingCampaign, Panel: models.PlacementPanelInstance, Pace: models.PlacementPaceSpaced,
		OnUnavailable: models.PlacementUnavailableDefer, SenderCount: 1, Status: models.PlacementBatchQueued,
		RetryUntil: time.Now().Add(time.Hour),
	}
	row, sender := uuid.New(), f.sender
	if err := batches.CreateBatch(ctx, b, []models.PlacementBatchSender{
		{ID: row, EmailAccountID: &sender, SenderEmail: "a@acme.test", SenderDomain: "acme.test", SenderFamily: "gmail"},
	}); err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}
	if ok, err := batches.ClaimBatchSender(ctx, row); err != nil || !ok {
		t.Fatalf("ClaimBatchSender = %v, %v", ok, err)
	}
	batchID := b.ID
	test := models.PlacementTest{
		ID: uuid.New(), OrganizationID: &f.org, SenderAccountID: &sender, SenderEmail: "a@acme.test",
		Subject: "Quick question", BodyPlain: "Hi there", Origin: models.PlacementOriginBatch,
		Panel: models.PlacementPanelInstance, Status: models.PlacementStatusRunning, BatchID: &batchID, BatchSenderID: &row,
	}
	seed := f.seeds[0]
	if err := f.repo.CreateTest(ctx, &test, []models.PlacementResult{{ID: uuid.New(), SeedAccountID: &seed, SeedAddress: "s@gmail.com", Family: "gmail", Folder: "pending"}}, nil); err != nil {
		t.Fatalf("CreateTest: %v", err)
	}

	ok, running, err := batches.CancelBatch(ctx, f.org, b.ID)
	if err != nil || !ok || len(running) != 1 || running[0] != test.ID {
		t.Fatalf("CancelBatch = %v, %v, %v; want the running test to cancel", ok, running, err)
	}
	late, err := batches.RunningTestsOfCancelledBatches(ctx, 1000)
	if err != nil || !containsOrgTest(late, test.ID) {
		t.Fatalf("RunningTestsOfCancelledBatches missed the running test (%v)", err)
	}
	if err := batches.SyncClosedBatchSenders(ctx); err != nil {
		t.Fatalf("SyncClosedBatchSenders: %v", err)
	}
	if p := progressOf(t, batches, b.ID); p.Running != 1 {
		t.Fatalf("progress %+v; want the sender running while its test is open", p)
	}
	f.exec(`UPDATE placement_tests SET status = 'cancelled', finished_at = NOW() WHERE id = $1`, test.ID)
	if err := batches.SyncClosedBatchSenders(ctx); err != nil {
		t.Fatalf("SyncClosedBatchSenders: %v", err)
	}
	if p := progressOf(t, batches, b.ID); p.Cancelled != 1 || p.Open() != 0 {
		t.Fatalf("progress %+v; want the sender resolved as cancelled", p)
	}
}

func containsOrgTest(ts []PlacementOrgTest, id uuid.UUID) bool {
	for _, t := range ts {
		if t.TestID == id {
			return true
		}
	}
	return false
}
