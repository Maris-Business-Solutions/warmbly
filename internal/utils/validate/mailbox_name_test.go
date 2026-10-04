package validate

import (
	"strings"
	"testing"
)

func TestMailboxNameShown(t *testing.T) {
	for _, ok := range []string{"Ada Lovelace", "José García-Núñez", strings.Repeat("Ana ", 20) + "Lee"} {
		if xerr := MailboxNameShown(ok); xerr != nil {
			t.Errorf("%q refused: %v", ok, xerr)
		}
	}
	for _, bad := range []string{"Visit evil.example.com", "ada@example.com", "https://x.test", "Ada‮Lee"} {
		if MailboxNameShown(bad) == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
