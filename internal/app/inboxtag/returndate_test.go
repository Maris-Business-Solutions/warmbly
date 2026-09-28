package inboxtag

import (
	"context"
	"testing"
	"time"

	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

// A workspace that holds a lead on an away message, which is what makes the
// return-date answer worth paying for.
func holdingSettings(on bool) fakeSettings {
	return fakeSettings{reply: models.ReplyIntentSettings{Enabled: true, HoldOnOutOfOffice: on}}
}

// An away message the offline layers decide, naming the first day of the
// absence: the misreading the question exists to catch.
func awayMessage() (Message, time.Time) {
	day := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, 21)
	m := inboundMessage()
	m.Subject = "Abwesenheitsnotiz: Ihre Anfrage"
	m.BodyText = "Ich bin ab dem " + day.Format("02.01.2006") + " nicht im Büro."
	m.PreviousMessage = "Hätten Sie am 3. Oktober Zeit für ein Gespräch?"
	return m, day
}

func TestOfflineAwayMessageIsAskedOnlyItsReturnDate(t *testing.T) {
	asker := &capturingAsker{resp: Response{Answers: map[string]Answer{QReturnDate: {Type: QuestionNoul, Noul: 0.08}}}}
	repo := &fakeRepo{}
	svc := newService(t, asker, repo)
	svc.WireSettings(holdingSettings(true))
	m, day := awayMessage()

	d, err := svc.Classify(context.Background(), m)
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if len(asker.questions) != 1 || asker.questions[QReturnDate].Type != QuestionNoul {
		t.Fatalf("asked %v, want only the return-date noul", asker.questions)
	}
	if want := "ab dem " + day.Format("02.01.2006"); asker.state.ReturnPhrase != want {
		t.Fatalf("return phrase = %q, want %q", asker.state.ReturnPhrase, want)
	}
	if asker.state.PreviousMessage != "" {
		t.Fatal("our previous send went into the state; its dates are not the sender's")
	}
	if d.Kind != KindAutoReplyOOO || d.KindSource != "header" || !d.ReturnDateAsked || !d.Automated() {
		t.Fatalf("decision = %+v", d)
	}
	saved := repo.saved[0]
	if saved.ReturnDate == nil || !saved.ReturnDate.Equal(day) {
		t.Fatalf("saved return date = %v, want %v", saved.ReturnDate, day)
	}
	if !ReturnDateDoubted(saved, day) {
		t.Fatal("a firm no on the stored verdict did not doubt the date")
	}
	if len(d.Labels) != 1 || d.Labels[0] != "Out of office" {
		t.Fatalf("labels = %v; the answer must not become a label", d.Labels)
	}
}

// Nothing reads the answer unless the workspace holds on out-of-office, and a
// parser that found no date has nothing to confirm.
func TestReturnDateIsAskedOnlyWhenAHoldReadsIt(t *testing.T) {
	cases := map[string]struct {
		settings *fakeSettings
		body     string
	}{
		"no settings":          {nil, ""},
		"hold off":             {ptr(holdingSettings(false)), ""},
		"reply automation off": {ptr(fakeSettings{reply: models.ReplyIntentSettings{HoldOnOutOfOffice: true}}), ""},
		"no date":              {ptr(holdingSettings(true)), "Ich bin zur Zeit nicht im Büro."},
	}
	for name, c := range cases {
		asker := &countingAsker{}
		repo := &fakeRepo{}
		svc := newService(t, asker, repo)
		if c.settings != nil {
			svc.WireSettings(*c.settings)
		}
		m, _ := awayMessage()
		if c.body != "" {
			m.BodyText = c.body
		}
		d, err := svc.Classify(context.Background(), m)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if asker.calls != 0 || d.ReturnDateAsked || repo.saved[0].ReturnDate != nil {
			t.Errorf("%s: %d calls, asked %v, saved date %v", name, asker.calls, d.ReturnDateAsked, repo.saved[0].ReturnDate)
		}
	}
}

func ptr[T any](v T) *T { return &v }

// A failed call leaves the verdict as it was before the question existed, so
// the parser's date stands.
func TestFailedReturnDateCallKeepsTheParsedDate(t *testing.T) {
	asker := &failingAsker{}
	repo := &fakeRepo{}
	svc := newService(t, asker, repo)
	svc.WireSettings(holdingSettings(true))
	m, day := awayMessage()

	d, err := svc.Classify(context.Background(), m)
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if asker.calls != 1 || d.Kind != KindAutoReplyOOO || len(repo.saved) != 1 {
		t.Fatalf("calls %d, decision %+v, saved %d", asker.calls, d, len(repo.saved))
	}
	if repo.saved[0].ReturnDate != nil || ReturnDateDoubted(repo.saved[0], day) {
		t.Fatal("an unanswered question doubted the date")
	}
}

// When the offline layers cannot tell what the message is, the question rides
// in the one full call and is read only if the model says it is an away
// message.
func TestReturnDateRidesTheFullCall(t *testing.T) {
	m, day := awayMessage()
	m.Subject = "Re: Ihre Anfrage"
	m.BodyText = "Danke! Ich bin ab dem " + day.Format("02.01.2006") + " wieder im Büro und melde mich dann."

	for _, tc := range []struct {
		kind  string
		asked bool
	}{{KindAutoReplyOOO, true}, {KindHumanReply, false}} {
		asker := &capturingAsker{resp: Response{Answers: map[string]Answer{
			"kind":      {Choice: tc.kind, Confidence: 0.95},
			"intent":    {Choice: IntentNotNow, Confidence: 0.9},
			QReturnDate: {Noul: 0.91},
		}}}
		repo := &fakeRepo{}
		svc := newService(t, asker, repo)
		svc.WireSettings(holdingSettings(true))

		d, err := svc.Classify(context.Background(), m)
		if err != nil {
			t.Fatalf("%s: %v", tc.kind, err)
		}
		if _, ok := asker.questions[QReturnDate]; !ok || len(asker.questions) < 2 || asker.state.ReturnPhrase == "" {
			t.Fatalf("%s: the question was not in the full call", tc.kind)
		}
		if d.ReturnDateAsked != tc.asked || (repo.saved[0].ReturnDate != nil) != tc.asked {
			t.Fatalf("%s: asked %v, saved date %v", tc.kind, d.ReturnDateAsked, repo.saved[0].ReturnDate)
		}
		if ReturnDateDoubted(repo.saved[0], day) {
			t.Fatalf("%s: a confirmed date was doubted", tc.kind)
		}
	}
}

// The backfill labels history and holds nobody, so it asks nothing for a hold.
func TestBackfillAsksNoReturnDate(t *testing.T) {
	asker := &countingAsker{}
	m, _ := awayMessage()
	repo := &fakeRepo{untagged: []repository.BackfillCandidate{{
		EmailAccountID: m.EmailAccountID,
		MessageID:      m.MessageID,
		ThreadID:       m.ThreadID,
		Subject:        m.Subject,
		BodyText:       m.BodyText,
		FromAddr:       m.FromAddr,
	}}}
	svc := newService(t, asker, repo)
	svc.WireSettings(holdingSettings(true))

	if _, err := svc.Backfill(context.Background(), m.OrganizationID, BackfillOptions{Limit: 5}); err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if asker.calls != 0 || repo.saved[0].ReturnDate != nil {
		t.Fatalf("backfill made %d calls for an away message", asker.calls)
	}
}

// Only a firm no about the same day, on an away-message verdict, doubts it.
func TestReturnDateDoubted(t *testing.T) {
	day := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	row := func(kind string, date time.Time, answers map[string]Answer) *repository.InboxTagResult {
		r := storedRow(kind, "", answers)
		r.ReturnDate = &date
		return r
	}
	no := map[string]Answer{QReturnDate: {Noul: 0.2}}
	cases := map[string]struct {
		r    *repository.InboxTagResult
		want bool
	}{
		"nil verdict":            {nil, false},
		"not asked":              {storedRow(KindAutoReplyOOO, "", nil), false},
		"firm no":                {row(KindAutoReplyOOO, day, no), true},
		"just under the floor":   {row(KindAutoReplyOOO, day, map[string]Answer{QReturnDate: {Noul: ReturnDateFloor - 0.01}}), true},
		"at the floor":           {row(KindAutoReplyOOO, day, map[string]Answer{QReturnDate: {Noul: ReturnDateFloor}}), false},
		"another day":            {row(KindAutoReplyOOO, day.AddDate(0, 0, 1), no), false},
		"not an away message":    {row(KindHumanReply, day, no), false},
		"date but no answer":     {row(KindAutoReplyOOO, day, nil), false},
		"another zone, same day": {row(KindAutoReplyOOO, day.In(time.FixedZone("CET", 3600)), no), true},
	}
	for name, c := range cases {
		if got := ReturnDateDoubted(c.r, day); got != c.want {
			t.Errorf("%s: doubted = %v, want %v", name, got, c.want)
		}
	}
}

// A message synced with no body is read by the hold from its snippet, so the
// tagger asks about the date in the snippet and shows the model that text.
func TestReturnDateReadsTheSnippetLikeTheHold(t *testing.T) {
	asker := &capturingAsker{resp: Response{Answers: map[string]Answer{QReturnDate: {Noul: 0.1}}}}
	repo := &fakeRepo{}
	svc := newService(t, asker, repo)
	svc.WireSettings(holdingSettings(true))
	m, day := awayMessage()
	m.Snippet, m.BodyText = m.BodyText, ""

	if _, err := svc.Classify(context.Background(), m); err != nil {
		t.Fatalf("classify: %v", err)
	}
	if asker.state.Body == "" || asker.state.ReturnPhrase == "" {
		t.Fatalf("state = %+v, want the snippet and its phrase", asker.state)
	}
	if !ReturnDateDoubted(repo.saved[0], day) {
		t.Fatal("the answer about the snippet's date was not stored for the hold")
	}
}
