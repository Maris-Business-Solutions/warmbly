package models

import "testing"

func TestClassifyWarmupLanding(t *testing.T) {
	cases := []struct {
		name   string
		folder string
		flags  []string
		want   string
	}{
		{"plain inbox", FolderInbox, nil, WarmupLandedInbox},
		{"gmail spam label", "", []string{"SPAM"}, WarmupLandedSpam},
		{"graph junk", FolderSpam, []string{"\\Junk"}, WarmupLandedSpam},
		{"imap junk folder without keyword", FolderSpam, []string{"\\Seen"}, WarmupLandedSpam},
		{"gmail promotions", FolderInbox, []string{"CATEGORY_PROMOTIONS"}, WarmupLandedTabs},
		{"gmail updates", "", []string{"CATEGORY_UPDATES"}, WarmupLandedTabs},
		{"gmail primary", "", []string{"CATEGORY_PERSONAL"}, WarmupLandedInbox},
		{"spam outranks a tab", "", []string{"CATEGORY_PROMOTIONS", "SPAM"}, WarmupLandedSpam},
	}
	for _, c := range cases {
		if got := ClassifyWarmupLanding(c.folder, c.flags); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestWarmupRecipientGroup(t *testing.T) {
	cases := []struct{ host, provider, want string }{
		{"google_workspace", "smtp_imap", WarmupRecipientGoogle},
		{"microsoft365", "smtp_imap", WarmupRecipientMicrosoft},
		{"outlook", "outlook", WarmupRecipientMicrosoft},
		{"aol", "smtp_imap", WarmupRecipientYahoo},
		{"", "gmail", WarmupRecipientGoogle},
		{"", "outlook", WarmupRecipientMicrosoft},
		{"", "smtp_imap", WarmupRecipientOther},
		{"zoho", "gmail", WarmupRecipientOther},
	}
	for _, c := range cases {
		if got := WarmupRecipientGroup(c.host, c.provider); got != c.want {
			t.Errorf("(%q, %q): got %q, want %q", c.host, c.provider, got, c.want)
		}
	}
}

func TestNewWarmupPlacementRate(t *testing.T) {
	if r := NewWarmupPlacementRate(0, 0, 0); r.Band != WarmupPlacementBandNone || r.InboxRate != nil {
		t.Fatalf("empty window: %+v", r)
	}
	if r := NewWarmupPlacementRate(12, 0, 0); r.Band != WarmupPlacementBandCollecting || r.InboxRate != nil {
		t.Fatalf("below the floor must withhold the rate: %+v", r)
	}
	r := NewWarmupPlacementRate(15, 3, 2)
	if r.InboxRate == nil || *r.InboxRate != 90 || r.Band != WarmupPlacementBandGood {
		t.Fatalf("tabs count as inbox: %+v", r)
	}
	if r := NewWarmupPlacementRate(17, 0, 3); r.Band != WarmupPlacementBandFair {
		t.Fatalf("85%% is fair: %+v", r)
	}
	if r := NewWarmupPlacementRate(15, 0, 5); r.Band != WarmupPlacementBandPoor {
		t.Fatalf("75%% is poor: %+v", r)
	}
}

// The headline is taken at the major providers while any of them received
// mail, and the other hosts ride beside it; with none it covers every host.
func TestWarmupPlacementWindowRate(t *testing.T) {
	w := WarmupPlacementWindow{
		Major: WarmupPlacementTally{Inbox: 18, Tabs: 2},
		All:   WarmupPlacementTally{Inbox: 28, Tabs: 2, Spam: 10},
	}
	r := w.Rate()
	if r.Scope != WarmupPlacementScopeMajor || r.Delivered != 20 || r.InboxRate == nil || *r.InboxRate != 100 {
		t.Fatalf("major rate = %+v, want 20 delivered at 100%%", r)
	}
	if r.OtherDelivered != 20 || r.OtherInboxRate == nil || *r.OtherInboxRate != 50 {
		t.Fatalf("other hosts = %d at %v, want 20 at 50%%", r.OtherDelivered, r.OtherInboxRate)
	}

	only := WarmupPlacementWindow{All: WarmupPlacementTally{Inbox: 15, Spam: 5}}.Rate()
	if only.Scope != WarmupPlacementScopeAll || only.Delivered != 20 || only.OtherDelivered != 0 || only.OtherInboxRate != nil {
		t.Fatalf("all-host rate = %+v, want every host and nothing beside it", only)
	}

	// Three Gmail deliveries do not hide a rate over 150 at small hosts.
	thin := WarmupPlacementWindow{Major: WarmupPlacementTally{Inbox: 3}, All: WarmupPlacementTally{Inbox: 120, Spam: 33}}.Rate()
	if thin.Scope != WarmupPlacementScopeAll || thin.InboxRate == nil || thin.Delivered != 153 {
		t.Fatalf("thin major rate = %+v, want the all-host figure", thin)
	}
	// With neither at the sample, the major count is what is being collected.
	early := WarmupPlacementWindow{Major: WarmupPlacementTally{Inbox: 3}, All: WarmupPlacementTally{Inbox: 8}}.Rate()
	if early.Scope != WarmupPlacementScopeMajor || early.Band != WarmupPlacementBandCollecting || early.Delivered != 3 {
		t.Fatalf("early rate = %+v, want 3 of the major sample collecting", early)
	}
}
