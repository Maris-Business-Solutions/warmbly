package tasks

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/app/cipher"
	"github.com/warmbly/warmbly/internal/app/unsublink"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

// fakeRenders stores frozen comparison copies the way placement_renders does:
// the first freeze per pair wins. hideReads makes every read miss, so both
// halves render and race to freeze.
type fakeRenders struct {
	repository.PlacementRepository
	rows      map[string]string
	freezes   int
	reads     int
	hideReads bool
}

func renderKey(group uuid.UUID, seed string) string {
	return group.String() + "|" + strings.ToLower(seed)
}

func (f *fakeRenders) GetPlacementRender(_ context.Context, _, group uuid.UUID, seed string) (string, error) {
	f.reads++
	if f.hideReads {
		return "", nil
	}
	return f.rows[renderKey(group, seed)], nil
}

func (f *fakeRenders) FreezePlacementRender(_ context.Context, _, group uuid.UUID, seed, content string) (string, error) {
	f.freezes++
	if f.rows == nil {
		f.rows = map[string]string{}
	}
	k := renderKey(group, seed)
	if _, ok := f.rows[k]; !ok {
		f.rows[k] = content
	}
	return f.rows[k], nil
}

type fakeTicketStore struct {
	repository.TrackedLinkRepository
	dest map[uuid.UUID]string
}

func (f *fakeTicketStore) CreateBatch(_ context.Context, links []repository.TrackedLink) error {
	for _, l := range links {
		f.dest[l.ID] = l.Destination
	}
	return nil
}

// compareBody varies in every sentence, nests spintax, and carries merge
// fields, an unsubscribe link, a stylesheet and two links to wrap.
const compareBody = `<style>p{margin:0}</style>` +
	`<p>{Hi|Hey|Hello|Morning} {{.FirstName}},</p>` +
	`<p>{quick question|wanted to ask|one thing|curious} about {{.Company}}: {are you|is your team|would you be} {open|free|around} {this week|next week|soon|later}?</p>` +
	`<p>{I|We} {built|made|ship} {a {tool|thing}|{software|an app}} {for|that helps} {sales|ops|founders|agencies}.</p>` +
	`<p>See <a href="https://example.com/pricing">pricing</a> or <a href="https://example.com/demo">{a demo|the demo}</a>.</p>` +
	`<p>{Cheers|Best|Thanks|Talk soon}</p>` +
	`<p>{{.UnsubscribeLink}}</p>`

func compareFixture(t *testing.T) (*tasksService, *fakeRenders, *fakeTicketStore, *models.Email, models.PlacementTest, models.PlacementTest) {
	t.Helper()
	renders := &fakeRenders{}
	tickets := &fakeTicketStore{dest: map[uuid.UUID]string{}}
	s := &tasksService{
		cipherService:   cipher.NewStatic([]byte("0123456789abcdef0123456789abcdef")),
		placementRepo:   renders,
		trackedLinkRepo: tickets,
		unsubLinks:      unsublink.New("secret", "https://api.example.com"),
	}
	account := &models.Email{
		Email:                  "ana@sender.com",
		SignatureSync:          true,
		SignatureHTML:          `<p>Ana, <a href="https://sender.com">sender.com</a></p>`,
		SignaturePlain:         "Ana, sender.com",
		TrackingDomain:         "track.sender.com",
		TrackingDomainVerified: true,
	}
	org, group := uuid.New(), uuid.New()
	untracked := models.PlacementTest{
		ID:             uuid.New(),
		OrganizationID: &org,
		CompareGroupID: &group,
		Subject:        "{Quick|Short|Brief|One} {question|idea|note} {for {{.FirstName}}|about {{.Company}}}",
		BodyHTML:       compareBody,
	}
	tracked := untracked
	tracked.ID = uuid.New()
	tracked.OpenTracking, tracked.LinkTracking = true, true
	return s, renders, tickets, account, untracked, tracked
}

var (
	pixelTag   = regexp.MustCompile(`<img[^>]*/t/o/[^>]*>`)
	ticketHref = regexp.MustCompile(`https://track\.sender\.com/c/([0-9a-f-]{36})`)
)

// untrack takes the pixel out of a tracked body and points every ticket back
// at its destination: what is left must be the untracked body exactly.
func untrack(body string, tickets *fakeTicketStore) string {
	body = pixelTag.ReplaceAllString(body, "")
	return ticketHref.ReplaceAllStringFunc(body, func(m string) string {
		return tickets.dest[uuid.MustParse(ticketHref.FindStringSubmatch(m)[1])]
	})
}

// Both halves of a comparison send one seed the same words; the tracked half
// differs only by its pixel and wrapped links.
func TestPlacementCompareHalvesDifferOnlyByTracking(t *testing.T) {
	s, _, tickets, account, untracked, tracked := compareFixture(t)
	ctx := context.Background()
	subjects := map[string]bool{}
	for i := range 24 {
		seed := "seed" + string(rune('a'+i)) + "@gmail.com"
		tracedTask := uuid.New()
		a, reason := s.renderPlacementProbe(ctx, uuid.New(), &untracked, account, seed)
		if reason != "" {
			t.Fatalf("untracked half refused: %s", reason)
		}
		b, reason := s.renderPlacementProbe(ctx, tracedTask, &tracked, account, seed)
		if reason != "" {
			t.Fatalf("tracked half refused: %s", reason)
		}
		subjects[a.Subject] = true

		if a.Subject != b.Subject {
			t.Fatalf("seed %s: subjects differ: %q vs %q", seed, a.Subject, b.Subject)
		}
		if a.BodyPlain != b.BodyPlain {
			t.Fatalf("seed %s: plain parts differ:\n%s\n---\n%s", seed, a.BodyPlain, b.BodyPlain)
		}
		if a.UnsubscribeURL == "" || a.UnsubscribeURL != b.UnsubscribeURL {
			t.Fatalf("seed %s: unsubscribe headers differ: %q vs %q", seed, a.UnsubscribeURL, b.UnsubscribeURL)
		}
		if got := untrack(b.BodyHTML, tickets); got != a.BodyHTML {
			t.Fatalf("seed %s: tracked body differs beyond tracking:\n%s\n---\n%s", seed, a.BodyHTML, got)
		}
		if !strings.Contains(b.BodyHTML, "/t/o/"+tracedTask.String()+".png") {
			t.Fatalf("seed %s: tracked half has no open pixel", seed)
		}
		if strings.Contains(a.BodyHTML, "/t/o/") || strings.Contains(a.BodyHTML, "track.sender.com/c/") {
			t.Fatalf("seed %s: untracked half carries tracking: %s", seed, a.BodyHTML)
		}
		if !strings.Contains(a.BodyHTML, `href="https://example.com/pricing"`) || strings.Contains(b.BodyHTML, `href="https://example.com/pricing"`) {
			t.Fatalf("seed %s: links not wrapped in the tracked half only", seed)
		}
		// The signature and the opt-out go on after tracking, as on a real send.
		if !strings.Contains(b.BodyHTML, `href="https://sender.com"`) || !strings.Contains(b.BodyHTML, a.UnsubscribeURL) {
			t.Fatalf("seed %s: signature or unsubscribe link was wrapped: %s", seed, b.BodyHTML)
		}
		if a.Tracking != nil || b.Tracking == nil || !b.Tracking.OpenTracking || !b.Tracking.LinkTracking {
			t.Fatalf("seed %s: tracking info wrong: %+v / %+v", seed, a.Tracking, b.Tracking)
		}
		if a.To[0] != seed || b.To[0] != seed || a.MessageID == b.MessageID {
			t.Fatalf("seed %s: halves must go to the same seed as separate messages", seed)
		}
		if strings.Contains(a.Subject, "{") || strings.Contains(a.BodyHTML, "{") {
			t.Fatalf("seed %s: spintax left unresolved: %q", seed, a.Subject)
		}
	}
	// The copy does vary between seeds, so equal halves are not luck.
	if len(subjects) < 2 {
		t.Fatalf("every seed got the same subject; the spintax never varied")
	}
}

// A retry, or a half that runs after the mailbox changed, sends what was
// frozen rather than rendering its own.
func TestPlacementCompareReusesTheFrozenCopy(t *testing.T) {
	s, renders, _, account, untracked, tracked := compareFixture(t)
	ctx := context.Background()
	const seed = "seed@outlook.com"

	first, reason := s.renderPlacementProbe(ctx, uuid.New(), &untracked, account, seed)
	if reason != "" {
		t.Fatal(reason)
	}
	account.SignatureHTML, account.SignaturePlain = "<p>Someone else</p>", "Someone else"
	for range 5 {
		again, reason := s.renderPlacementProbe(ctx, uuid.New(), &untracked, account, seed)
		if reason != "" {
			t.Fatal(reason)
		}
		if again.Subject != first.Subject || again.BodyHTML != first.BodyHTML || again.BodyPlain != first.BodyPlain {
			t.Fatalf("a retry rendered its own copy:\n%s\n---\n%s", first.BodyHTML, again.BodyHTML)
		}
	}
	other, _ := s.renderPlacementProbe(ctx, uuid.New(), &tracked, account, strings.ToUpper(seed))
	if other.Subject != first.Subject || other.BodyPlain != first.BodyPlain {
		t.Fatalf("the sibling rendered its own copy: %q vs %q", other.Subject, first.Subject)
	}
	if renders.freezes != 1 {
		t.Fatalf("froze %d copies for one seed, want 1", renders.freezes)
	}
}

// Two halves that render at the same moment both send the copy that was
// stored first.
func TestPlacementCompareRaceAdoptsTheStoredCopy(t *testing.T) {
	s, renders, _, account, untracked, tracked := compareFixture(t)
	renders.hideReads = true
	ctx := context.Background()
	for _, seed := range []string{"a@gmail.com", "b@gmail.com", "c@gmail.com", "d@gmail.com"} {
		a, _ := s.renderPlacementProbe(ctx, uuid.New(), &untracked, account, seed)
		b, _ := s.renderPlacementProbe(ctx, uuid.New(), &tracked, account, seed)
		if a.Subject == "" || a.Subject != b.Subject || a.BodyPlain != b.BodyPlain {
			t.Fatalf("racing halves sent different copy: %q vs %q", a.Subject, b.Subject)
		}
	}
}

// A copy that cannot be sent is refused for both halves, so a seed never
// gets one half alone.
func TestPlacementCompareFreezesARefusal(t *testing.T) {
	s, _, _, account, untracked, tracked := compareFixture(t)
	ctx := context.Background()
	untracked.BodyHTML, tracked.BodyHTML = "<div></div>", "<div></div>"
	if _, reason := s.renderPlacementProbe(ctx, uuid.New(), &untracked, account, "seed@yahoo.com"); reason != "The copy rendered empty" {
		t.Fatalf("reason = %q", reason)
	}
	tracked.BodyHTML = "<p>Now there is copy</p>"
	if _, reason := s.renderPlacementProbe(ctx, uuid.New(), &tracked, account, "seed@yahoo.com"); reason != "The copy rendered empty" {
		t.Fatalf("the sibling rendered on its own after its pair was refused: %q", reason)
	}
}

// A single test has no pair to share a copy with, so nothing is stored.
func TestPlacementSingleTestStoresNothing(t *testing.T) {
	s, renders, _, account, untracked, _ := compareFixture(t)
	untracked.CompareGroupID = nil
	if _, reason := s.renderPlacementProbe(context.Background(), uuid.New(), &untracked, account, "seed@gmail.com"); reason != "" {
		t.Fatal(reason)
	}
	if renders.reads != 0 || renders.freezes != 0 {
		t.Fatalf("a single test touched the comparison store: %d reads, %d freezes", renders.reads, renders.freezes)
	}
}

// With no key to seal it under, a comparison sends nothing rather than
// storing message content in the clear.
func TestPlacementCompareNeedsTheWorkspaceKey(t *testing.T) {
	s, renders, _, account, untracked, _ := compareFixture(t)
	s.cipherService = nil
	if _, reason := s.renderPlacementProbe(context.Background(), uuid.New(), &untracked, account, "seed@gmail.com"); reason == "" {
		t.Fatal("a comparison sent without a key to seal its copy")
	}
	if renders.freezes != 0 {
		t.Fatal("stored a copy without sealing it")
	}
}
