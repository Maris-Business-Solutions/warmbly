package replyclassify

import "testing"

func TestClassifySystemReports(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   Input
		want bool
	}{
		{"dmarc without headers", Input{Subject: "Report Domain: example.test Submitter: seznam.cz Report-ID: 123"}, true},
		{"case insensitive", Input{Subject: "  report domain: example.test submitter: google.com report-id: abc"}, true},
		{"report before autoresponder header", Input{Subject: "Report Domain: example.test Submitter: google.com Report-ID: abc", Headers: map[string][]string{"Auto-Submitted": {"auto-replied"}}}, true},
		{"tls json", Input{Headers: map[string][]string{"content-type": {"application/tlsrpt+json"}}}, true},
		{"tls multipart", Input{Headers: map[string][]string{"Content-Type": {`multipart/report; report-type="tlsrpt"; boundary=123`}}}, true},
		{"human discussing a report", Input{Subject: "Re: Report Domain: example.test Submitter: google.com Report-ID: abc", BodyText: "Sure, happy to chat."}, false},
		{"ordinary dmarc question", Input{Subject: "Can you help with our DMARC report?", BodyText: "Sure, happy to chat."}, false},
		{"incomplete subject", Input{Subject: "Report Domain: example.test"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsSystemReport(tc.in); got != tc.want {
				t.Fatalf("IsSystemReport = %v, want %v", got, tc.want)
			}
			got := ClassifyOffline(tc.in)
			if tc.want && (got.Class != ClassAutoReply || got.Source != SourceHeader) {
				t.Fatalf("report classified as %+v", got)
			}
			if !tc.want && IsAutomated(got.Class) {
				t.Fatalf("human mail classified as %+v", got)
			}
		})
	}
}

// Layer 1 decides "automated" for the whole pipeline, and an automated verdict
// means replied_at is never stamped. Both directions therefore matter, and the
// second is the dangerous one: a provider's away-message subject has to be
// caught in the languages a European list actually answers in (issue #470),
// while a human reply must never be caught by any of it.
func TestClassifyOutOfOfficeSubjects(t *testing.T) {
	automated := []string{
		"Automatic reply: Quick question",
		"Out of office",
		"Out of Office AutoReply: Quick question",
		"Abwesenheitsnotiz",
		"Automatische Antwort: Ihre Nachricht",
		"Abwesend bis 15.09.2026",
		"Réponse automatique : votre message",
		"Respuesta automática",
		"Risposta automatica: la tua email",
		"Automatisch antwoord",
		"Autoreply",
		"Auto: away until Monday",
		"Automatyczna odpowiedź",
	}
	for _, subject := range automated {
		t.Run(subject, func(t *testing.T) {
			got := Classify(Input{Subject: subject})
			if got.Class != ClassOutOfOffice {
				t.Fatalf("Classify(%q).Class = %q, want %q", subject, got.Class, ClassOutOfOffice)
			}
		})
	}
}

// A reply from a person arrives as "Re: <our own campaign subject>", so the
// away-message vocabulary must never be matched anywhere but the start of the
// subject. Reading one of these as automated would stop replied_at being
// stamped, stop_on_reply would never fire, and this feature would then hold and
// keep emailing somebody who actually answered — the exact failure issue #470
// is about, inverted.
func TestClassifyNeverCatchesOurOwnSubjectEchoedBack(t *testing.T) {
	subjects := []string{
		"Re: Quick question",
		"Re: Out of office cover for Jane",
		"Re: Cutting annual leave admin at Acme",
		"Re: Get away from the inbox grind",
		"Re: How does your parental leave policy scale?",
		"AW: Abwesenheit der Kollegin vertreten",
		"Antw: Arbeiten Sie außer Haus?",
		"Re: Still on holiday pricing?",
	}
	for _, subject := range subjects {
		t.Run(subject, func(t *testing.T) {
			got := Classify(Input{Subject: subject, BodyText: "Thanks, not the right time for us."})
			if IsAutomated(got.Class) {
				t.Fatalf("Classify(%q) = %q: a reply carrying our own subject must never read as automated",
					subject, got.Class)
			}
		})
	}
}

// A person answering a cold email must not be swept up by the away-message
// vocabulary in the BODY either.
func TestClassifyLeavesHumanRepliesAlone(t *testing.T) {
	human := []struct{ subject, body string }{
		{"Re: Quick question", "Sure, happy to chat next week."},
		{"Re: Quick question", "I'm on holiday next week, ping me after."},
		{"Re: Quick question", "Thanks, not the right time for us."},
		{"Re: Ihre Nachricht", "Danke, im Moment kein Bedarf."},
	}
	for _, c := range human {
		t.Run(c.subject+"/"+c.body[:min(20, len(c.body))], func(t *testing.T) {
			got := Classify(Input{Subject: c.subject, BodyText: c.body})
			if IsAutomated(got.Class) {
				t.Fatalf("Classify(%q, %q) = %q, a human reply must never read as automated",
					c.subject, c.body, got.Class)
			}
		})
	}
}

// A real Exchange or Gmail autoresponder carries RFC 3834 / vendor headers, so
// an away message whose subject convention is not in the table is still caught.
func TestClassifyStillReadsAutoSubmittedHeaders(t *testing.T) {
	got := Classify(Input{
		Subject: "Re: Quick question",
		Headers: map[string][]string{"Auto-Submitted": {"auto-replied"}},
	})
	if got.Class != ClassOutOfOffice {
		t.Fatalf("Auto-Submitted: auto-replied classified as %q, want %q", got.Class, ClassOutOfOffice)
	}
}
