package emailsend

import "testing"

func TestCheckHeaderValuesRefusesInjection(t *testing.T) {
	cases := []struct {
		name string
		req  SendEmailRequest
		code string
	}{
		{"crlf in to", SendEmailRequest{To: []string{"a@b.com\r\nBcc: c@d.com"}}, ErrCodeInvalidRecipient},
		{"two addresses in one entry", SendEmailRequest{To: []string{"a@b.com, c@d.com"}}, ErrCodeInvalidRecipient},
		{"bad cc", SendEmailRequest{To: []string{"a@b.com"}, CC: []string{"not an address"}}, ErrCodeInvalidRecipient},
		{"lf in bcc", SendEmailRequest{To: []string{"a@b.com"}, BCC: []string{"c@d.com\nX-Evil: 1"}}, ErrCodeInvalidRecipient},
		{"no to", SendEmailRequest{To: []string{" "}}, ErrCodeInvalidRecipient},
		{"crlf in in_reply_to", SendEmailRequest{To: []string{"a@b.com"}, InReplyTo: []string{"<x@y>\r\nBcc: c@d.com"}}, ErrCodeInvalidMessageID},
		{"space in in_reply_to", SendEmailRequest{To: []string{"a@b.com"}, InReplyTo: []string{"<x@y> <z@w>"}}, ErrCodeInvalidMessageID},
	}
	for _, tc := range cases {
		xerr := checkHeaderValues(&tc.req)
		if xerr == nil || xerr.Identifier != tc.code {
			t.Errorf("%s: got %v, want %s", tc.name, xerr, tc.code)
		}
	}
}

func TestCheckHeaderValuesAcceptsAndTidies(t *testing.T) {
	req := SendEmailRequest{
		To:        []string{"Ana <ana@example.com>", ""},
		CC:        []string{"cc@example.com"},
		InReplyTo: []string{" <abc@mail.example.com> ", ""},
	}
	if xerr := checkHeaderValues(&req); xerr != nil {
		t.Fatalf("refused a valid request: %v", xerr)
	}
	if len(req.To) != 1 || len(req.InReplyTo) != 1 || req.InReplyTo[0] != "<abc@mail.example.com>" {
		t.Fatalf("got to=%q in_reply_to=%q", req.To, req.InReplyTo)
	}
}
