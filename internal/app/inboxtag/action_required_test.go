package inboxtag

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

// A no-reply notice the header layer decides offline.
func billingNotice() Message {
	return Message{
		OrganizationID: uuid.New(),
		EmailAccountID: uuid.New(),
		MessageID:      "<billing@google.com>",
		ThreadID:       "t-billing",
		Subject:        "Payment declined for your Google Workspace subscription",
		BodyText:       "We could not charge your card. Update your payment details or your account will be suspended.",
		FromAddr:       "Google Workspace <payments-noreply@google.com>",
	}
}

func noticeSettings(on bool, qs ...models.InboxTagQuestion) fakeSettings {
	return fakeSettings{s: models.InboxTaggingSettings{ActionRequiredInInbox: on, Questions: qs}}
}

func invoiceQuestion(automated bool) models.InboxTagQuestion {
	return models.InboxTagQuestion{
		ID:        "q9",
		Type:      models.InboxTagQuestionYesNo,
		Question:  "The message is an invoice.",
		Label:     "Invoice",
		Automated: automated,
	}
}

type failingAsker struct{ calls int }

func (f *failingAsker) Ask(context.Context, any, map[string]Question) (*Response, error) {
	f.calls++
	return nil, errors.New("typesafe unavailable")
}

func TestActionRequiredKeepsANotificationInTheInbox(t *testing.T) {
	d := Decide(map[string]Answer{
		"kind":            {Choice: KindNotification, Confidence: 0.95},
		SigActionRequired: {Noul: 0.9},
	}, Facts{})
	if !d.ActionRequired || d.Automated() {
		t.Fatalf("action required = %v, automated = %v", d.ActionRequired, d.Automated())
	}
	if !slices.Equal(d.Labels, []string{"Notification", LabelActionRequired}) {
		t.Fatalf("labels = %v", d.Labels)
	}
	if d.Priority != PriorityToday {
		t.Fatalf("priority = %q (relevance %d), want it with the replies due today", d.Priority, d.Relevance)
	}
}

func TestRoutineNotificationStaysAutomated(t *testing.T) {
	d := Decide(map[string]Answer{SigActionRequired: {Noul: 0.3}}, Facts{DeterministicKind: KindNotification})
	if d.ActionRequired || !d.Automated() || slices.Contains(d.Labels, LabelActionRequired) {
		t.Fatalf("a receipt was kept in the inbox: %+v", d)
	}
	if d.SignalStrength[SigActionRequired] != 0.3 {
		t.Fatalf("the answer was not recorded: %v", d.SignalStrength)
	}
}

// Only a notification is asked: a bounce or an auto-reply is about our own
// sending, and a person's reply already reaches the inbox.
func TestActionRequiredReadsOnlyNotifications(t *testing.T) {
	for _, kind := range []string{KindBounceHard, KindAutoReplyOOO, KindAutoReplyTicket, KindHumanReply, KindColdInbound} {
		d := Decide(map[string]Answer{SigActionRequired: {Noul: 0.99}}, Facts{DeterministicKind: kind})
		if d.ActionRequired || d.KeepInInbox || slices.Contains(d.Labels, LabelActionRequired) {
			t.Errorf("%s: %+v", kind, d)
		}
	}
	// An untrusted kind stays in the inbox without the label.
	d := Decide(map[string]Answer{
		"kind":            {Choice: KindNotification, Confidence: 0.4},
		SigActionRequired: {Noul: 0.99},
	}, Facts{})
	if d.ActionRequired || d.Automated() {
		t.Fatalf("untrusted kind: %+v", d)
	}
}

func TestOfflineNotificationIsAskedOnlyTheActionQuestion(t *testing.T) {
	asker := &capturingAsker{resp: Response{Answers: map[string]Answer{SigActionRequired: {Noul: 0.92}}}}
	repo := &fakeRepo{}
	svc := newService(t, asker, repo)
	svc.WireSettings(noticeSettings(true, invoiceQuestion(false)))

	d, err := svc.Classify(context.Background(), billingNotice())
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if len(asker.questions) != 1 {
		t.Fatalf("asked %d questions of a notice, want only the action check: %v", len(asker.questions), asker.questions)
	}
	if _, ok := asker.questions[SigActionRequired]; !ok {
		t.Fatal("the action check was not asked")
	}
	if d.Kind != KindNotification || d.KindSource != "header" || !d.ActionRequired {
		t.Fatalf("decision = %+v", d)
	}
	if len(repo.saved) != 1 || repo.saved[0].Automated || !slices.Contains(repo.saved[0].Labels, LabelActionRequired) {
		t.Fatalf("saved = %+v", repo.saved)
	}
}

func TestOfflineNotificationWithTheCheckOffMakesNoCall(t *testing.T) {
	asker := &countingAsker{}
	repo := &fakeRepo{}
	svc := newService(t, asker, repo)
	svc.WireSettings(noticeSettings(false, invoiceQuestion(false)))

	d, err := svc.Classify(context.Background(), billingNotice())
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if asker.calls != 0 {
		t.Fatalf("made %d calls with nothing to ask", asker.calls)
	}
	if !d.Automated() || len(repo.saved) != 1 || !repo.saved[0].Automated {
		t.Fatalf("decision %+v, saved %+v", d, repo.saved)
	}
}

// A failed check files the notice as it was filed before the check existed,
// unasked, so an outage does not fill every inbox with newsletters and the
// notification re-check asks it later.
func TestFailedActionCheckFilesTheNoticeUnasked(t *testing.T) {
	asker := &failingAsker{}
	repo := &fakeRepo{}
	svc := newService(t, asker, repo)

	d, err := svc.Classify(context.Background(), billingNotice())
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if asker.calls != 1 || !d.Automated() || len(repo.saved) != 1 || !repo.saved[0].Automated {
		t.Fatalf("calls %d, decision %+v, saved %+v", asker.calls, d, repo.saved)
	}
	if string(repo.saved[0].Answers) != "{}" {
		t.Fatalf("answers = %s, want none so the re-check offers it again", repo.saved[0].Answers)
	}
}

func TestWorkspaceQuestionAskedOfNotifications(t *testing.T) {
	asker := &capturingAsker{resp: Response{Answers: map[string]Answer{
		SigActionRequired: {Noul: 0.1},
		"custom_q9":       {Noul: 0.85},
	}}}
	repo := &fakeRepo{}
	svc := newService(t, asker, repo)
	svc.WireSettings(noticeSettings(true, invoiceQuestion(true), laterMaybe(models.InboxTagQuestionAction{})))

	d, err := svc.Classify(context.Background(), billingNotice())
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if _, ok := asker.questions["custom_q1"]; ok {
		t.Fatal("a question about replies was asked of a notification")
	}
	if len(asker.questions) != 2 {
		t.Fatalf("questions = %v", asker.questions)
	}
	if d.ActionRequired || !d.KeepInInbox || d.Automated() || !slices.Contains(d.Labels, "Invoice") {
		t.Fatalf("decision = %+v", d)
	}
	if len(repo.saved) != 1 || repo.saved[0].Automated {
		t.Fatalf("saved = %+v", repo.saved)
	}
}

// Asked of notifications does not mean asked of bounces, and never acts.
func TestWorkspaceNotificationQuestionScope(t *testing.T) {
	stop := invoiceQuestion(true)
	stop.Action = models.InboxTagQuestionAction{Type: models.InboxTagActionStop}
	custom := []models.InboxTagQuestion{stop}
	answers := map[string]Answer{"custom_q9": {Noul: 0.95}}

	if d := DecideWith(answers, Facts{DeterministicKind: KindBounceHard}, custom); len(d.Custom) != 0 || d.KeepInInbox {
		t.Fatalf("bounce: %+v", d)
	}
	d := DecideWith(answers, Facts{DeterministicKind: KindNotification}, custom)
	if len(d.Custom) != 1 || !d.KeepInInbox {
		t.Fatalf("notification: %+v", d)
	}
	if p := PlanActions(d, allOn()); !p.Empty() {
		t.Fatalf("a notification acted: %+v", p)
	}
	// And it still reads replies as before.
	reply := map[string]Answer{"kind": {Choice: KindHumanReply, Confidence: 0.95}, "custom_q9": {Noul: 0.95}}
	if d := DecideWith(reply, Facts{}, custom); len(d.Custom) != 1 || d.KeepInInbox {
		t.Fatalf("reply: %+v", d)
	}
}

func TestFullCallCarriesTheActionCheckOnlyWhenOn(t *testing.T) {
	if _, ok := QuestionsFor(nil, true)[SigActionRequired]; !ok {
		t.Fatal("on: the action check is missing from the full call")
	}
	if _, ok := QuestionsFor(nil, false)[SigActionRequired]; ok {
		t.Fatal("off: the action check was still asked")
	}
	if len(NotificationQuestions([]models.InboxTagQuestion{invoiceQuestion(false)}, false)) != 0 {
		t.Fatal("off with no notification questions still asks something")
	}
}

func TestRecheckNotificationsReclassifiesStoredNotices(t *testing.T) {
	asker := &capturingAsker{resp: Response{Answers: map[string]Answer{SigActionRequired: {Noul: 0.9}}}}
	n := billingNotice()
	repo := &fakeRepo{notices: []repository.BackfillCandidate{{
		EmailAccountID: n.EmailAccountID,
		MessageID:      n.MessageID,
		ThreadID:       n.ThreadID,
		Subject:        n.Subject,
		BodyText:       n.BodyText,
		FromAddr:       n.FromAddr,
	}}}
	svc := newService(t, asker, repo)

	p, err := svc.Backfill(context.Background(), uuid.New(), BackfillOptions{RecheckNotifications: true})
	if err != nil {
		t.Fatalf("recheck: %v", err)
	}
	if p.Classified != 1 || !slices.Equal(repo.reopened, []string{n.MessageID}) {
		t.Fatalf("progress %+v, reopened %v", p, repo.reopened)
	}
	if len(repo.saved) != 1 || repo.saved[0].Automated {
		t.Fatalf("saved = %+v", repo.saved)
	}
}

// Nothing to read is skipped rather than reopened, or it would come back
// unasked and be offered on every run.
func TestRecheckNotificationsSkipsEmptyNotices(t *testing.T) {
	asker := &countingAsker{}
	repo := &fakeRepo{notices: []repository.BackfillCandidate{{MessageID: "<empty@notice>", FromAddr: "no-reply@vendor.test"}}}
	svc := newService(t, asker, repo)

	p, err := svc.Backfill(context.Background(), uuid.New(), BackfillOptions{RecheckNotifications: true})
	if err != nil {
		t.Fatalf("recheck: %v", err)
	}
	if p.Skipped != 1 || asker.calls != 0 || len(repo.reopened) != 0 {
		t.Fatalf("progress %+v, calls %d, reopened %v", p, asker.calls, repo.reopened)
	}
}

func TestRecheckNotificationsRefusesWhenNothingIsAsked(t *testing.T) {
	svc := newService(t, &countingAsker{}, &fakeRepo{})
	// A question asked of notifications alone does not write the check's answer.
	svc.WireSettings(noticeSettings(false, invoiceQuestion(true)))
	if _, err := svc.Backfill(context.Background(), uuid.New(), BackfillOptions{RecheckNotifications: true}); !errors.Is(err, ErrNothingToAsk) {
		t.Fatalf("err = %v, want ErrNothingToAsk", err)
	}
}

// Without the built-in check nothing would re-ask a notice filed unasked, so a
// failed call for the workspace's own questions releases it for a backfill.
func TestFailedNotificationQuestionReleasesWithTheCheckOff(t *testing.T) {
	asker := &failingAsker{}
	repo := &fakeRepo{}
	svc := newService(t, asker, repo)
	svc.WireSettings(noticeSettings(false, invoiceQuestion(true)))

	m := billingNotice()
	if _, err := svc.Classify(context.Background(), m); err == nil {
		t.Fatal("a failed call was not reported")
	}
	if len(repo.saved) != 0 || repo.tagged[m.MessageID] {
		t.Fatalf("saved %v, claim kept %v", repo.saved, repo.tagged[m.MessageID])
	}
}

// A recheck whose call failed asked nothing, and says so.
func TestRecheckNotificationsCountsAFailedCheckAsFailed(t *testing.T) {
	n := billingNotice()
	repo := &fakeRepo{notices: []repository.BackfillCandidate{{
		EmailAccountID: n.EmailAccountID, MessageID: n.MessageID, ThreadID: n.ThreadID,
		Subject: n.Subject, BodyText: n.BodyText, FromAddr: n.FromAddr,
	}}}
	svc := newService(t, &failingAsker{}, repo)

	p, err := svc.Backfill(context.Background(), uuid.New(), BackfillOptions{RecheckNotifications: true})
	if err != nil {
		t.Fatalf("recheck: %v", err)
	}
	if p.Failed != 1 || p.Classified != 0 {
		t.Fatalf("progress = %+v, want the unasked recheck counted as failed", p)
	}
}
