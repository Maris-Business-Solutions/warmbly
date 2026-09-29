package models

import (
	"testing"
	"unicode/utf8"
)

func TestEmailMessageStoreDataValidText(t *testing.T) {
	m := &EmailMessageStoreData{
		Subject:  "caf\xe9 \xe2\xa2\x77",
		Snippet:  "a\x00b",
		BodyText: "ok ✓",
		FromAddr: []string{"Ren\xe9 <rene@example.com>"},
	}
	m.ValidText()

	for _, s := range append([]string{m.Subject, m.Snippet, m.BodyText}, m.FromAddr...) {
		if !utf8.ValidString(s) {
			t.Fatalf("still invalid: %q", s)
		}
	}
	if m.Snippet != "ab" {
		t.Fatalf("NUL kept: %q", m.Snippet)
	}
	if m.BodyText != "ok ✓" {
		t.Fatalf("valid text changed: %q", m.BodyText)
	}
}
