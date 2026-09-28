package imap

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/emersion/go-imap/v2"
)

// The sync pass and warmup actions share one session. A warmup action selects
// its own folder between two sync steps; the sync step after it must still
// read the folder the sync selected, not the one the action left behind.
func TestSyncStepReopensItsFolderAfterAWarmupAction(t *testing.T) {
	c := testServer(t, imap.CapSet{imap.CapIMAP4rev1: {}, imap.CapMove: {}}, "Archive")
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	appendMessage(t, c, "Archive", "<a1@test>")
	appendMessage(t, c, "Archive", "<a2@test>")
	appendMessage(t, c, "INBOX", "<w1@test>")
	appendMessage(t, c, "INBOX", "<w2@test>")

	if _, err := c.SelectForSync("Archive"); err != nil {
		t.Fatalf("select Archive: %v", err)
	}
	if _, err := c.MoveToFolder(context.Background(), "INBOX", "Warmbly", 1); err != nil {
		t.Fatalf("file warmup: %v", err)
	}
	fetched, err := c.FetchEnvelopes(context.Background(), []imap.UID{2})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(fetched) != 1 || fetched[0].Email.MessageID != "a2@test" {
		var got []string
		for _, f := range fetched {
			got = append(got, f.Email.MessageID)
		}
		t.Fatalf("the sync step read the folder the warmup action selected: got %v, want [a2@test]", got)
	}
}

// Every warmup filing lands while the sync pass keeps re-selecting other
// folders on the same session, which is the steady state of a mailbox without
// CONDSTORE: a flag scan selects every folder on every pass.
func TestWarmupFilingLandsWhileTheSyncWalksFolders(t *testing.T) {
	c := testServer(t, imap.CapSet{imap.CapIMAP4rev1: {}, imap.CapMove: {}}, "Archive", "Warmbly")
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	const n = 30
	for i := 1; i <= n; i++ {
		appendMessage(t, c, "INBOX", fmt.Sprintf("<w%d@test>", i))
	}
	appendMessage(t, c, "Archive", "<a1@test>")

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			for _, box := range []string{"Archive", "INBOX"} {
				if _, err := c.SelectForSync(box); err != nil {
					continue
				}
				_, _ = c.FetchFlags(context.Background(), 1)
			}
		}
	}()

	for uid := uint32(1); uid <= n; uid++ {
		if _, err := c.MoveToFolder(context.Background(), "INBOX", "Warmbly", uid); err != nil {
			t.Errorf("file uid %d: %v", uid, err)
		}
	}
	close(stop)
	wg.Wait()

	count, err := c.SelectForSync("INBOX")
	if err != nil {
		t.Fatalf("select INBOX: %v", err)
	}
	if count != 0 {
		t.Fatalf("%d of %d warmup messages were left in the inbox", count, n)
	}
	if count, err = c.SelectForSync("Warmbly"); err != nil || count != n {
		t.Fatalf("Warmbly holds %d messages (err %v), want %d", count, err, n)
	}
}

// A folder recreated under the sync must fail every later step, not only the
// first: forgetting the view would send the rest of the batch to whatever
// folder a warmup action selected.
func TestSyncStepsRefuseARecreatedFolderUntilReselected(t *testing.T) {
	c := testServer(t, imap.CapSet{imap.CapIMAP4rev1: {}, imap.CapMove: {}}, "Archive")
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	appendMessage(t, c, "INBOX", "<w1@test>")
	appendMessage(t, c, "INBOX", "<w2@test>")
	appendMessage(t, c, "Archive", "<a1@test>")

	if _, err := c.SelectForSync("Archive"); err != nil {
		t.Fatalf("select Archive: %v", err)
	}
	c.lifecycle.RLock()
	for _, step := range []func() error{
		func() error { return c.client.Unselect().Wait() },
		func() error { return c.client.Delete("Archive").Wait() },
		func() error { return c.client.Create("Archive", nil).Wait() },
	} {
		if err := step(); err != nil {
			c.lifecycle.RUnlock()
			t.Fatalf("recreate Archive: %v", err)
		}
	}
	c.lifecycle.RUnlock()
	appendMessage(t, c, "Archive", "<new@test>")

	for i := range 2 {
		if _, err := c.MoveToFolder(context.Background(), "INBOX", "Warmbly", uint32(i+1)); err != nil {
			t.Fatalf("file warmup: %v", err)
		}
		fetched, err := c.FetchEnvelopes(context.Background(), []imap.UID{1, 2})
		if err == nil {
			t.Fatalf("step %d read %d messages from a folder whose UIDs it no longer holds", i+1, len(fetched))
		}
	}
	if _, err := c.SelectForSync("Archive"); err != nil {
		t.Fatalf("reselect Archive: %v", err)
	}
	if fetched, err := c.FetchEnvelopes(context.Background(), []imap.UID{1}); err != nil || len(fetched) != 1 || fetched[0].Email.MessageID != "new@test" {
		t.Fatalf("after an explicit select the sync reads the new folder: %v %v", fetched, err)
	}
}
