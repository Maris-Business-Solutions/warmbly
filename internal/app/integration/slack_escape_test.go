package integration

import "testing"

func TestSlackEscapeKeepsLinksAndMentionsAsText(t *testing.T) {
	in := "Re: <https://evil.example|Reset your password> & <!channel>"
	want := "Re: &lt;https://evil.example|Reset your password&gt; &amp; &lt;!channel&gt;"
	if got := slackEscape(in); got != want {
		t.Errorf("slackEscape = %q, want %q", got, want)
	}
}
