package wmail

import "testing"

func TestBuildSendHeadersReplyTo(t *testing.T) {
	tests := []struct {
		name string
		req  *SendRequest
		want string
	}{
		{name: "campaign send", req: &SendRequest{ReplyTo: "replies@acme.com"}, want: "replies@acme.com"},
		{name: "display name reduced to the address", req: &SendRequest{ReplyTo: "Replies <replies@acme.com>"}, want: "replies@acme.com"},
		{name: "none configured", req: &SendRequest{}, want: ""},
		// A warmup reply is read back in the sending mailbox.
		{name: "warmup send", req: &SendRequest{IsWarmup: true, WarmupToken: "tok", ReplyTo: "replies@acme.com"}, want: ""},
		{name: "header injection", req: &SendRequest{ReplyTo: "replies@acme.com\r\nBcc: x@evil.test"}, want: ""},
		{name: "two addresses", req: &SendRequest{ReplyTo: "a@acme.com, b@acme.com"}, want: ""},
		{name: "not an address", req: &SendRequest{ReplyTo: "replies"}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildSendHeaders(tt.req)["Reply-To"]
			if got != tt.want {
				t.Fatalf("Reply-To = %q, want %q", got, tt.want)
			}
		})
	}
}
