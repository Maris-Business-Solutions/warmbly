package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// The Gmail folder reconciliation reads the rows Gmail last had in a folder,
// newest first, for one mailbox, and only those it can look up by Gmail id.
//
//	WARMBLY_TEST_DB=postgres://warmbly:warmbly@localhost:15432/<db>?sslmode=disable \
//	  go test ./internal/repository/ -run LiveProviderFolderMessages -v
func TestLiveProviderFolderMessages(t *testing.T) {
	d, pool := liveContactDB(t)
	f := newThreadParentFixture(t, pool)
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(),
			`DELETE FROM unibox_emails WHERE email_id IN (SELECT id FROM email_accounts WHERE organization_id = $1)`, f.org); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})

	now := time.Now().UTC().Truncate(time.Second)
	row := func(mailbox uuid.UUID, gmailID, folder, providerFolder string, age time.Duration) uuid.UUID {
		id := uuid.New()
		f.exec(`INSERT INTO unibox_emails (id, user_id, email_id, gmail_id, folder, provider_folder, internal_date)
		        VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			id, f.owner, mailbox, gmailID, folder, providerFolder, now.Add(-age))
		return id
	}
	newest := row(f.mailbox, "g-new", "inbox", "inbox", time.Hour)
	filedHere := row(f.mailbox, "g-filed", "archive", "inbox", 2*time.Hour)
	archived := row(f.mailbox, "g-arch", "archive", "archive", 3*time.Hour)
	row(f.mailbox, "g-sent", "sent", "sent", 30*time.Minute)
	row(f.mailbox, "", "inbox", "inbox", 10*time.Minute)
	row(f.other, "g-other", "inbox", "inbox", 5*time.Minute)

	repo := NewEmailSyncStateRepository(d)
	got, err := repo.ListProviderFolderMessages(context.Background(), f.owner, f.mailbox, []string{"inbox", "archive"}, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	want := []uuid.UUID{newest, filedHere, archived}
	if len(got) != len(want) {
		t.Fatalf("got %d rows, want %d: %+v", len(got), len(want), got)
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Errorf("row %d = %s, want %s", i, got[i].ID, id)
		}
	}
	if got[1].ProviderID != "g-filed" || got[1].ProviderFolder != "inbox" {
		t.Errorf("filed row = %+v, want Gmail id g-filed in provider folder inbox", got[1])
	}

	limited, err := repo.ListProviderFolderMessages(context.Background(), f.owner, f.mailbox, []string{"inbox", "archive"}, 1)
	if err != nil || len(limited) != 1 || limited[0].ID != newest {
		t.Fatalf("limit 1 = %+v (%v), want only the newest row", limited, err)
	}
}
