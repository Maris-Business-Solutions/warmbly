package repository

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/models"
)

// Contacts copied on one lead's emails (issue #731). The hold that keeps a
// copied contact from getting a second sequence lives in triggers, so it is
// asserted against the real routing query.
//
//	WARMBLY_TEST_DB=postgres://warmbly:warmbly@localhost:15432/<db>?sslmode=disable \
//	  go test ./internal/repository/ -run LiveLeadCC -v

func routedIDs(pairs []ContactSequencePair) map[uuid.UUID]bool {
	out := map[uuid.UUID]bool{}
	for _, p := range pairs {
		out[p.ContactID] = true
	}
	return out
}

func TestLiveLeadCCHoldsTheCopiedLeadUntilReleased(t *testing.T) {
	_, pool := liveContactDB(t)
	f := newRoutedPairsFixture(t, pool, 3)
	repo := NewCampaignProgressRepository(pool)
	ctx := context.Background()
	a, b, c := f.leads[0], f.leads[1], f.leads[2]

	if err := repo.SetLeadCC(ctx, f.org, f.campaign, a, []uuid.UUID{b}); err != nil {
		t.Fatalf("SetLeadCC: %v", err)
	}
	pairs, _, _ := f.find(t, nil, 25)
	got := routedIDs(pairs)
	if !got[a] || got[b] || !got[c] {
		t.Fatalf("routed %v; want the lead and the uncopied lead, not the copied one", got)
	}
	hold, err := repo.GetLeadHold(ctx, f.campaign, b)
	if err != nil || hold == nil || hold.Source != models.LeadHoldSourceCC {
		t.Fatalf("copied lead's hold = %+v, %v; want a cc hold", hold, err)
	}
	// Nothing is left to wait for, so a campaign of only copied leads can finish.
	if n, err := repo.CountHeldLeads(ctx, f.campaign); err != nil || n != 0 {
		t.Fatalf("CountHeldLeads = %d, %v; want 0 for a cc hold", n, err)
	}

	cc, err := repo.ListLeadCC(ctx, f.campaign, a)
	if err != nil || len(cc) != 1 || cc[0].ContactID != b || !cc[0].Copied() {
		t.Fatalf("ListLeadCC = %+v, %v; want the copy, active", cc, err)
	}

	if err := repo.SetLeadCC(ctx, f.org, f.campaign, a, nil); err != nil {
		t.Fatalf("SetLeadCC to nobody: %v", err)
	}
	if hold, err := repo.GetLeadHold(ctx, f.campaign, b); err != nil || hold != nil {
		t.Fatalf("hold after release = %+v, %v; want none", hold, err)
	}
	pairs, _, _ = f.find(t, nil, 25)
	if !routedIDs(pairs)[b] {
		t.Fatal("a released copy was not routed again")
	}
}

// A contact copied first and enrolled afterwards is held on arrival, whichever
// path enrols them.
func TestLiveLeadCCHoldsAContactEnrolledAfterBeingCopied(t *testing.T) {
	_, pool := liveContactDB(t)
	f := newRoutedPairsFixture(t, pool, 2)
	repo := NewCampaignProgressRepository(pool)
	ctx := context.Background()
	a, b := f.leads[0], f.leads[1]

	if _, err := pool.Exec(ctx, `DELETE FROM campaign_leads WHERE campaign_id = $1 AND contact_id = $2`, f.campaign, b); err != nil {
		t.Fatalf("drop lead: %v", err)
	}
	if err := repo.SetLeadCC(ctx, f.org, f.campaign, a, []uuid.UUID{b}); err != nil {
		t.Fatalf("SetLeadCC: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO campaign_leads (campaign_id, contact_id) VALUES ($1, $2)`, f.campaign, b); err != nil {
		t.Fatalf("enrol: %v", err)
	}
	if hold, err := repo.GetLeadHold(ctx, f.campaign, b); err != nil || hold == nil || hold.Source != models.LeadHoldSourceCC {
		t.Fatalf("hold on enrol = %+v, %v; want a cc hold", hold, err)
	}
	// Removing the lead that copies them releases them too.
	if _, err := pool.Exec(ctx, `DELETE FROM campaign_leads WHERE campaign_id = $1 AND contact_id = $2`, f.campaign, a); err != nil {
		t.Fatalf("drop copying lead: %v", err)
	}
	if hold, err := repo.GetLeadHold(ctx, f.campaign, b); err != nil || hold != nil {
		t.Fatalf("hold after the copying lead left = %+v, %v; want none", hold, err)
	}
}

func TestLiveLeadCCRefusals(t *testing.T) {
	_, pool := liveContactDB(t)
	f := newRoutedPairsFixture(t, pool, 3)
	other := newRoutedPairsFixture(t, pool, 1)
	repo := NewCampaignProgressRepository(pool)
	ctx := context.Background()
	a, b, c := f.leads[0], f.leads[1], f.leads[2]

	cases := []struct {
		name string
		lead uuid.UUID
		cc   []uuid.UUID
		want error
	}{
		{"self", a, []uuid.UUID{a}, ErrLeadCCSelf},
		{"another workspace's contact", a, []uuid.UUID{other.leads[0]}, ErrLeadCCContactNotFound},
		{"not a lead", other.leads[0], nil, ErrLeadNotInCampaign},
	}
	for _, tc := range cases {
		if err := repo.SetLeadCC(ctx, f.org, f.campaign, tc.lead, tc.cc); !errors.Is(err, tc.want) {
			t.Fatalf("%s: err = %v, want %v", tc.name, err, tc.want)
		}
	}

	if err := repo.SetLeadCC(ctx, f.org, f.campaign, a, []uuid.UUID{b}); err != nil {
		t.Fatalf("SetLeadCC: %v", err)
	}
	if err := repo.SetLeadCC(ctx, f.org, f.campaign, b, []uuid.UUID{c}); !errors.Is(err, ErrLeadCCLeadIsCopied) {
		t.Fatalf("copies on a copied lead: err = %v, want ErrLeadCCLeadIsCopied", err)
	}
	if err := repo.SetLeadCC(ctx, f.org, f.campaign, c, []uuid.UUID{a}); !errors.Is(err, ErrLeadCCHasCopies) {
		t.Fatalf("copying a lead with copies: err = %v, want ErrLeadCCHasCopies", err)
	}
	// A refused write leaves the list as it was.
	if cc, _ := repo.ListLeadCC(ctx, f.campaign, a); len(cc) != 1 || cc[0].ContactID != b {
		t.Fatalf("list after refusals = %+v; want it unchanged", cc)
	}
}

func TestLiveLeadCCStatusAndBounces(t *testing.T) {
	_, pool := liveContactDB(t)
	f := newRoutedPairsFixture(t, pool, 3)
	repo := NewCampaignProgressRepository(pool)
	ctx := context.Background()
	a, b, c := f.leads[0], f.leads[1], f.leads[2]

	if err := repo.SetLeadCC(ctx, f.org, f.campaign, a, []uuid.UUID{b, c}); err != nil {
		t.Fatalf("SetLeadCC: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE contacts SET subscribed = false WHERE id = $1`, b); err != nil {
		t.Fatalf("unsubscribe: %v", err)
	}
	var cEmail string
	if err := pool.QueryRow(ctx, `SELECT email FROM contacts WHERE id = $1`, c).Scan(&cEmail); err != nil {
		t.Fatalf("read email: %v", err)
	}
	id, err := repo.MarkLeadCCBounced(ctx, f.campaign, a, strings.ToUpper(cEmail))
	if err != nil || id == nil || *id != c {
		t.Fatalf("MarkLeadCCBounced = %v, %v; want the copy", id, err)
	}
	if id, err := repo.MarkLeadCCBounced(ctx, f.campaign, a, "nobody@test.local"); err != nil || id != nil {
		t.Fatalf("MarkLeadCCBounced on a stranger = %v, %v; want nil", id, err)
	}

	cc, err := repo.ListLeadCC(ctx, f.campaign, a)
	if err != nil || len(cc) != 2 {
		t.Fatalf("ListLeadCC = %+v, %v", cc, err)
	}
	want := map[uuid.UUID]string{b: models.LeadCCStatusUnsubscribed, c: models.LeadCCStatusBounced}
	for _, x := range cc {
		if x.Status != want[x.ContactID] || x.Copied() {
			t.Fatalf("copy %s status %q; want %q and not copied", x.ContactID, x.Status, want[x.ContactID])
		}
	}
}

func TestLiveLeadForCopiedReplyFindsTheLeadsThread(t *testing.T) {
	_, pool := liveContactDB(t)
	f := newRoutedPairsFixture(t, pool, 2)
	repo := NewCampaignProgressRepository(pool)
	ctx := context.Background()
	a, b := f.leads[0], f.leads[1]

	if err := repo.SetLeadCC(ctx, f.org, f.campaign, a, []uuid.UUID{b}); err != nil {
		t.Fatalf("SetLeadCC: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE campaign_leads SET email_account_id = $3 WHERE campaign_id = $1 AND contact_id = $2`,
		f.campaign, a, f.mailbox); err != nil {
		t.Fatalf("bind sender: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO campaign_contact_progress (campaign_id, contact_id, sequence_id, sent_at)
		VALUES ($1, $2, $3, NOW())`, f.campaign, a, f.step); err != nil {
		t.Fatalf("progress: %v", err)
	}

	ref, err := repo.LeadForCopiedReply(ctx, b, f.mailbox)
	if err != nil || ref == nil || ref.CampaignID != f.campaign || ref.ContactID != a || ref.SequenceID != f.step {
		t.Fatalf("LeadForCopiedReply = %+v, %v; want the lead's step", ref, err)
	}
	// Another mailbox never wrote to the lead, so it is no evidence.
	if ref, err := repo.LeadForCopiedReply(ctx, b, uuid.New()); err != nil || ref != nil {
		t.Fatalf("LeadForCopiedReply from another mailbox = %+v, %v; want nil", ref, err)
	}
}

// The Leads list carries each lead's copies, and a copied lead reads paused.
func TestLiveLeadCCShowsInTheLeadsList(t *testing.T) {
	handle, pool := liveContactDB(t)
	f := newRoutedPairsFixture(t, pool, 2)
	repo := NewCampaignProgressRepository(pool)
	contacts := NewContactRepostory(handle)
	ctx := context.Background()
	a, b := f.leads[0], f.leads[1]

	if err := repo.SetLeadCC(ctx, f.org, f.campaign, a, []uuid.UUID{b}); err != nil {
		t.Fatalf("SetLeadCC: %v", err)
	}
	res, xerr := contacts.Search(ctx, f.org.String(), nil, nil, models.SearchContacts{
		CampaignIDs: []string{f.campaign.String()},
	}, 25)
	if xerr != nil {
		t.Fatalf("search: %v", xerr)
	}
	byID := map[uuid.UUID]*models.ContactCampaignProgress{}
	for i := range res.Data {
		byID[res.Data[i].ID] = res.Data[i].CampaignLead
	}
	if lead := byID[a]; lead == nil || len(lead.CC) != 1 || lead.CC[0].ContactID != b || lead.CC[0].Status != models.LeadCCStatusActive {
		t.Fatalf("the copying lead reads %+v; want its one active copy", lead)
	}
	if lead := byID[b]; lead == nil || lead.Status != models.LeadStatusPaused || lead.Hold == nil || lead.Hold.Source != models.LeadHoldSourceCC {
		t.Fatalf("the copied lead reads %+v; want paused with a cc hold", lead)
	}
}
