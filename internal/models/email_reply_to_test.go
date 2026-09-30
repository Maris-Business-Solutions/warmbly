package models

import "testing"

func TestEmailReplyToHeader(t *testing.T) {
	tests := []struct {
		name    string
		account Email
		want    string
	}{
		{name: "unset", account: Email{Email: "b@acme.com"}, want: ""},
		{name: "another mailbox", account: Email{Email: "b@acme.com", ReplyTo: " a@acme.com "}, want: "a@acme.com"},
		{name: "own address", account: Email{Email: "b@acme.com", ReplyTo: "B@acme.com"}, want: ""},
		{name: "send-as alias", account: Email{Email: "b@acme.com", SendAsEmail: "hello@acme.com", ReplyTo: "hello@acme.com"}, want: ""},
		{name: "display name", account: Email{Email: "b@acme.com", ReplyTo: "Sales <a@acme.com>"}, want: "a@acme.com"},
		{name: "unparseable", account: Email{Email: "b@acme.com", ReplyTo: "not an address"}, want: ""},
		{name: "line break", account: Email{Email: "b@acme.com", ReplyTo: "a@acme.com\r\nBcc: x@evil.test"}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.account.ReplyToHeader(); got != tt.want {
				t.Fatalf("ReplyToHeader() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEmailReceivesAt(t *testing.T) {
	a := Email{Email: "a@acme.com", SendAsEmail: "sales@acme.com"}
	for addr, want := range map[string]bool{
		"a@acme.com":     true,
		" A@ACME.com ":   true,
		"sales@acme.com": true,
		"b@acme.com":     false,
		"":               false,
	} {
		if got := a.ReceivesAt(addr); got != want {
			t.Errorf("ReceivesAt(%q) = %v, want %v", addr, got, want)
		}
	}
}
