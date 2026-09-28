package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/models"
)

// A reply from an address that is not the contact's (an alias, a forward)
// still resolves to the lead the campaign wrote to, through the thread.
//
//	WARMBLY_TEST_DB=postgres://warmbly:warmbly@localhost:15432/<db>?sslmode=disable \
//	  go test ./internal/repository/ -run LiveContactThreadLookup -v

func newContactThreadLookupFixture(t *testing.T) (*threadParentFixture, ContactRepository) {
	t.Helper()
	handle, pool := liveContactDB(t)
	f := newThreadParentFixture(t, pool)
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(),
			`DELETE FROM unibox_emails WHERE email_id IN (SELECT id FROM email_accounts WHERE organization_id = $1)`, f.org); err != nil {
			t.Errorf("cleanup unibox: %v", err)
		}
	})
	return f, NewContactRepostory(handle)
}

func threadRef(id string, account *uuid.UUID, allowed ...uuid.UUID) models.ContactLookupThread {
	return models.ContactLookupThread{ID: id, AccountID: account, AllowedAccounts: allowed}
}

func TestLiveContactThreadLookupThroughTheSentCopy(t *testing.T) {
	f, repo := newContactThreadLookupFixture(t)
	ctx := context.Background()
	step := f.step(0, "Hello", false)
	f.send(step, f.mailbox, "<sent-1@test.local>", "", 600)
	now := time.Now().UTC()
	f.unibox(f.mailbox, "sent", "sent-1@test.local", "imap-thread", "me@test.local", nil, now.Add(-10*time.Minute))
	f.unibox(f.mailbox, "inbox", "<reply@alias.test>", "imap-thread", "Alias <alias@alias.test>", nil, now.Add(-5*time.Minute))

	got, xerr := repo.GetByThreadAndOrganization(ctx, f.org, threadRef("imap-thread", nil))
	if xerr != nil || got == nil || got.ID != f.contact {
		t.Fatalf("got %+v (%v), want the campaign's lead", got, xerr)
	}
	mb, other := f.mailbox, f.other
	if got, _ := repo.GetByThreadAndOrganization(ctx, f.org, threadRef("imap-thread", &mb)); got == nil || got.ID != f.contact {
		t.Fatalf("scoped to the mailbox: got %+v", got)
	}
	if got, _ := repo.GetByThreadAndOrganization(ctx, f.org, threadRef("imap-thread", nil, mb)); got == nil || got.ID != f.contact {
		t.Fatalf("allowlist naming the mailbox: got %+v", got)
	}

	// A mailbox that does not hold the thread, an allowlist without it, another
	// organization, or an unknown thread resolve nothing.
	if got, _ := repo.GetByThreadAndOrganization(ctx, f.org, threadRef("imap-thread", &other)); got != nil {
		t.Fatalf("another mailbox resolved %+v", got)
	}
	if got, _ := repo.GetByThreadAndOrganization(ctx, f.org, threadRef("imap-thread", nil, other)); got != nil {
		t.Fatalf("an allowlist without the mailbox resolved %+v", got)
	}
	if got, _ := repo.GetByThreadAndOrganization(ctx, uuid.New(), threadRef("imap-thread", nil)); got != nil {
		t.Fatalf("another organization resolved %+v", got)
	}
	if got, _ := repo.GetByThreadAndOrganization(ctx, f.org, threadRef("no-such-thread", nil)); got != nil {
		t.Fatalf("an unknown thread resolved %+v", got)
	}
	if got, _ := repo.GetByThreadAndOrganization(ctx, f.org, threadRef("", nil)); got != nil {
		t.Fatalf("an empty thread resolved %+v", got)
	}
}

// With no sent copy synced, a Message-ID the reply names is enough, even when
// the reply lands in another workspace mailbox than the one that sent.
func TestLiveContactThreadLookupThroughInReplyTo(t *testing.T) {
	f, repo := newContactThreadLookupFixture(t)
	ctx := context.Background()
	step := f.step(0, "Hello", false)
	f.send(step, f.mailbox, "<sent-2@test.local>", "", 600)
	f.unibox(f.other, "inbox", "<r1@alias.test>", "reply-only", "alias@alias.test", []string{"<sent-2@test.local>"}, time.Now().UTC())

	got, xerr := repo.GetByThreadAndOrganization(ctx, f.org, threadRef("reply-only", nil))
	if xerr != nil || got == nil || got.ID != f.contact {
		t.Fatalf("got %+v (%v), want the campaign's lead", got, xerr)
	}
}

// On Gmail the thread handle the worker recorded on the send is enough, but
// only for a mailbox that holds the thread.
func TestLiveContactThreadLookupThroughTheGmailThread(t *testing.T) {
	f, repo := newContactThreadLookupFixture(t)
	ctx := context.Background()
	step := f.step(0, "Hello", false)
	f.send(step, f.other, "<minted@gmail.com>", "gthr-9", 500)
	f.unibox(f.other, "inbox", "<r2@alias.test>", "gthr-9", "alias@alias.test", nil, time.Now().UTC())

	if got, xerr := repo.GetByThreadAndOrganization(ctx, f.org, threadRef("gthr-9", nil)); xerr != nil || got == nil || got.ID != f.contact {
		t.Fatalf("got %+v (%v), want the campaign's lead", got, xerr)
	}
	mb := f.mailbox
	if got, _ := repo.GetByThreadAndOrganization(ctx, f.org, threadRef("gthr-9", &mb)); got != nil {
		t.Fatalf("a handle from a mailbox without the thread resolved %+v", got)
	}
}
