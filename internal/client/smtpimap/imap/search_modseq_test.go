package imap

import (
	"strings"
	"testing"

	"github.com/emersion/go-imap/v2"
)

// zohoModSeqSearch answers a CONDSTORE search the way imappro.zoho.in does
// when nothing changed: the plain form carries a double space go-imap cannot
// parse, the ESEARCH form is well formed.
func zohoModSeqSearch(line string) string {
	fields := strings.Fields(line)
	if len(fields) < 2 || !strings.Contains(strings.ToUpper(line), "SEARCH") || !strings.Contains(strings.ToUpper(line), "MODSEQ") {
		return ""
	}
	tag := fields[0]
	if strings.Contains(strings.ToUpper(line), "RETURN") {
		return `* ESEARCH (TAG "` + tag + `") UID MODSEQ 1791314919556115300` + "\r\n" + tag + " OK Success"
	}
	return "* SEARCH  (MODSEQ 1791314919556115300)\r\n" + tag + " OK Success"
}

// Zoho dropped every SMTP/IMAP mailbox with "server unreachable" on the first
// quiet sync pass: the empty MODSEQ search answer killed the session.
func TestSearchChangedSinceOnZohoEmptyResult(t *testing.T) {
	caps := imap.CapSet{imap.CapIMAP4rev1: {}, imap.CapESearch: {}, imap.CapCondStore: {}}
	c, wire := interceptingServer(t, caps, zohoModSeqSearch)
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if _, err := c.SelectForSync("INBOX"); err != nil {
		t.Fatalf("SelectForSync: %v", err)
	}
	uids, err := c.SearchChangedSince(1791314919556115300)
	if err != nil {
		t.Fatalf("SearchChangedSince: %v", err)
	}
	if len(uids) != 0 {
		t.Fatalf("uids = %v, want none", uids)
	}
	searches := wire.commands("SEARCH")
	if len(searches) != 1 || !strings.Contains(searches[0], "RETURN (ALL)") {
		t.Fatalf("modseq search did not use ESEARCH: %v", searches)
	}
}
