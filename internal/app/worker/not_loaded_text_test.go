package worker

import (
	"testing"

	"github.com/warmbly/warmbly/internal/pkg/emailverify"
)

// A mailbox that never loaded says nothing about the recipient.
func TestMailboxNotLoadedIsNotRecipientEvidence(t *testing.T) {
	if emailverify.NamesRecipient(errMailboxNotLoaded) {
		t.Fatalf("%q reads as a recipient refusal", errMailboxNotLoaded)
	}
}
