package replyclassify

import "testing"

func TestIsOptOut(t *testing.T) {
	cases := []struct {
		name    string
		subject string
		body    string
		want    bool
	}{
		{"plain", "", "Please unsubscribe me from this list.", true},
		{"remove me", "Re: hi", "remove me from your list", true},
		{"stop emailing", "", "Stop emailing me.", true},
		{"hyphen", "", "I'd like to opt-out.", true},
		{"word boundary", "", "Stop by our booth next week!", false},
		{"substring", "", "We have an unsubscribed model in beta", false},
		{"quoted footer only", "", "Sounds interesting, let's talk.\n\nOn Tue, Sep 1, 2026 Jane <jane@x.com> wrote:\n> If this isn't relevant, just reply and I'll stop.\n> Unsubscribe: https://example.com/u/abc", false},
		{"quote lines only", "", "Yes please\n> unsubscribe here", false},
		{"before quote", "", "Please remove me\n\nOn Mon Jane wrote:\n> hello", true},
		{"outlook header", "", "not interested\r\nFrom: Jane\r\nSent: Monday\r\nunsubscribe", false},
		{"curly apostrophe", "", "Please don\u2019t email me again.", true},
		{"empty", "", "", false},
	}
	for _, c := range cases {
		if got := IsOptOut(c.subject, c.body); got != c.want {
			t.Errorf("%s: IsOptOut=%v want %v", c.name, got, c.want)
		}
	}
}

func TestLexiconIgnoresQuotedHistory(t *testing.T) {
	r, ok := classifyLexicon(Input{BodyText: "Sounds good, let's talk.\n\nOn Mon, X wrote:\n> reply STOP or unsubscribe to opt out"})
	if !ok || r.Class != ClassPositive {
		t.Fatalf("got %+v ok=%v, want positive", r, ok)
	}
}

// German opt-out wording is read on every reply, whatever languages the
// workspace tags in, and never from the quoted original: a German campaign
// step carries its own "bitte austragen" line.
func TestIsOptOutGerman(t *testing.T) {
	const ownLine = "Kein Interesse? Antworten Sie einfach mit \u201ebitte austragen\u201c."
	optOut := []string{
		"Bitte austragen.",
		"Bitte tragen Sie mich aus Ihrem Verteiler aus.",
		"Bitte tragen Sie uns aus.",
		"Trag mich bitte aus.",
		"Ich möchte mich abmelden.",
		"Bitte melden Sie mich ab.",
		"Bitte um Abmeldung.",
		"Ich möchte keine weiteren E-Mails von Ihnen erhalten.",
		"Keine weitere Mail, danke.",
		"Bitte entfernen Sie meine Adresse aus Ihrer Datenbank.",
		"Bitte nimm mich raus.",
		"Bitte löschen Sie meine Daten.",
		"Bitte loeschen Sie meine Daten.",
		"Löschen Sie bitte meine Daten.",
		"Schreib mir bitte nicht mehr.",
		"Bitte schreiben Sie mich nicht mehr an.",
		"Bitte sehen Sie von weiteren Kontaktaufnahmen ab.",
		"Unterlassen Sie künftig jegliche Kontaktaufnahme.",
		"Keine Werbemails mehr bitte.",
		"Bitte stoppen Sie die Zusendungen.",
		"Hiermit widerspreche ich der Verwendung meiner Daten zu Werbezwecken.",
		"Ich widerspreche hiermit der Verarbeitung meiner personenbezogenen Daten.",
		"BITTE MICH NICHT MEHR KONTAKTIEREN",
		"Bitte löschen Sie mich aus Ihrer Datenbank.",
		"Keine weitere Kontaktaufnahme erwünscht.",
		"Bitte aus dem Verteiler nehmen.",
		"Bitte von der Liste streichen.",
		"Ich möchte keine weiteren\r\nE-Mails erhalten.",
		"Bitte austragen.\r\n\r\nVon: Marc <marc@example.com>\r\nGesendet: Montag, 28. September 2026 09:12\r\n\r\n" + ownLine,
	}
	for _, body := range optOut {
		if !IsOptOut("", body) {
			t.Errorf("IsOptOut(%q) = false, want true", body)
		}
	}
	if !IsOptOut("Abmelden bitte", "") {
		t.Error("an opt-out subject with an empty body was missed")
	}

	notOptOut := map[string]string{
		"interest":        "Klingt spannend, rufen Sie mich gerne an.",
		"meeting":         "Ich muss mich leider vom Termin am Freitag abmelden, passt Montag?",
		"calendar":        "Ich musste den Termin leider wieder austragen.",
		"participle":      "Ich bin heute abgemeldet, melde mich morgen.",
		"consent":         "Mit der Verwendung meiner Daten für die Terminbuchung bin ich einverstanden.",
		"disagree":        "Da widerspreche ich Ihnen, unser Vertrieb läuft gut.",
		"no ads":          "Wir machen aktuell keine Werbung, aber gerne ein Termin.",
		"colleague":       "Herrn Müller brauchen Sie nicht mehr kontaktieren, ich bin jetzt zuständig.",
		"forwarded":       "Ich habe Ihre Mail an die Kollegen aus dem Verteiler weitergeleitet.",
		"contact person":  "Für weitere Kontaktaufnahmen wenden Sie sich bitte an Frau Klein.",
		"from monday":     "Ich meld mich ab Montag wieder, dann gerne ein Termin.",
		"pilot":           "Melden Sie mich ab Oktober für den Pilot an.",
		"call instead":    "Bitte kontaktieren Sie mich nicht per Mail, sondern rufen Sie an.",
		"not before":      "Kontaktieren Sie uns nicht vor Oktober, dann gerne.",
		"quote item":      "Streichen Sie uns bitte Position 3 aus dem Angebot.",
		"print ads":       "Wir machen keine Werbung mehr in Print, aber online gerne.",
		"no mail yet":     "Ich habe bisher keine weiteren Nachrichten erhalten.",
		"disclaimer":      "Gerne, passt.\n\nDiese E-Mail enthält vertrauliche Informationen. Wenn Sie nicht der richtige Adressat sind, informieren Sie bitte sofort den Absender und löschen Sie diese E-Mail.",
		"gmail quote":     "Klingt gut, melden Sie sich.\n\nAm Mo., 28. Sept. 2026 um 09:12 Uhr schrieb Marc <marc@example.com>:\n> " + ownLine,
		"outlook quote":   "Klingt interessant, rufen Sie mich an.\r\n\r\nVon: Marc <marc@example.com>\r\nGesendet: Montag, 28. September 2026 09:12\r\nAn: Jan <jan@example.de>\r\nBetreff: Kurze Frage\r\n\r\n" + ownLine,
		"outlook divider": "Passt, gerne.\r\n\r\n-----Ursprüngliche Nachricht-----\r\n" + ownLine,
		"gmx quote":       "Klingt interessant.\r\n\r\nGesendet: Montag, 28. September 2026 um 09:12 Uhr\r\nVon: \"Marc\" <marc@example.com>\r\nAn: jan@example.de\r\nBetreff: Kurze Frage\r\n\r\n" + ownLine,
		"t-online quote":  "Gerne, rufen Sie an.\r\n\r\n-------- Original-Nachricht --------\r\n" + ownLine,
	}
	for name, body := range notOptOut {
		if IsOptOut("", body) {
			t.Errorf("%s: IsOptOut(%q) = true, want false", name, body)
		}
	}
}

func TestLexiconGermanOptOut(t *testing.T) {
	r, ok := classifyLexicon(Input{Subject: "AW: Kurze Frage", BodyText: "Bitte austragen, danke."})
	if !ok || r.Class != ClassUnsubscribe {
		t.Fatalf("got %+v ok=%v, want unsubscribe", r, ok)
	}
	r, ok = classifyLexicon(Input{BodyText: "Sounds good, let's talk.\r\n\r\nVon: Marc <marc@example.com>\r\nGesendet: Montag, 28. September 2026 09:12\r\n\r\nKein Interesse? Einfach \u201ebitte austragen\u201c antworten."})
	if !ok || r.Class != ClassPositive {
		t.Fatalf("got %+v ok=%v, want positive", r, ok)
	}
}
