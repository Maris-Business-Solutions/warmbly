package imap

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
)

// quotedSearchDate is a SEARCH date key followed by a quoted date, the form
// Seznam.cz refuses and go-imap always writes.
var quotedSearchDate = regexp.MustCompile(`(?i)\b(SINCE|BEFORE|ON|SENTSINCE|SENTBEFORE|SENTON) "`)

// seznamRefusal answers a dated SEARCH the way imap.seznam.cz does (#660).
func seznamRefusal(line string) string {
	fields := strings.Fields(line)
	if len(fields) < 2 || !strings.Contains(strings.ToUpper(line), "SEARCH") || !quotedSearchDate.MatchString(line) {
		return ""
	}
	return fields[0] + " NO SEARCH: mtd: internal error: Bad token (expecting date specification)."
}

// putDated appends one message to INBOX with the given internal date.
func putDated(t *testing.T, c *Client, at time.Time) imap.UID {
	t.Helper()
	raw := "From: sender@test\r\nTo: warmbly@test\r\nSubject: dated\r\n\r\nbody\r\n"
	c.lifecycle.RLock()
	defer c.lifecycle.RUnlock()
	cmd := c.client.Append("INBOX", int64(len(raw)), &imap.AppendOptions{Time: at})
	if _, err := cmd.Write([]byte(raw)); err != nil {
		t.Fatalf("append write: %v", err)
	}
	if err := cmd.Close(); err != nil {
		t.Fatalf("append close: %v", err)
	}
	data, err := cmd.Wait()
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	return data.UID
}

// Issue #660: Seznam.cz refuses a quoted SEARCH date, so the backfill never
// started. The window has to come back anyway, and match what SINCE means.
func TestSearchSinceOnServerRefusingQuotedDates(t *testing.T) {
	c, wire := interceptingServer(t, nil, seznamRefusal)
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	since := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	putDated(t, c, since.AddDate(0, 0, -30))
	sameDay := putDated(t, c, since.Add(90*time.Minute))
	putDated(t, c, since.Add(-time.Minute))
	recent := putDated(t, c, since.AddDate(0, 0, 10))

	if _, err := c.SelectForSync("INBOX"); err != nil {
		t.Fatalf("SelectForSync: %v", err)
	}
	got, err := c.SearchSince(since)
	if err != nil {
		t.Fatalf("SearchSince on a server refusing quoted dates: %s", err.Message)
	}
	if want := []imap.UID{sameDay, recent}; !slices.Equal(got, want) {
		t.Fatalf("SearchSince = %v, want %v", got, want)
	}
	if len(wire.commands("SEARCH")) != 1 {
		t.Fatalf("expected exactly the one refused SEARCH, got %v", wire.commands("SEARCH"))
	}
}

// The backfill asks again every pass. The refusal is remembered, and only the
// mail above what was already read is fetched again.
func TestSearchSinceByDateExtendsInsteadOfRescanning(t *testing.T) {
	c, wire := interceptingServer(t, nil, seznamRefusal)
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	since := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	putDated(t, c, since.AddDate(0, 0, -5))
	first := putDated(t, c, since.AddDate(0, 0, 1))

	if _, err := c.SelectForSync("INBOX"); err != nil {
		t.Fatalf("SelectForSync: %v", err)
	}
	if got, err := c.SearchSince(since); err != nil || !slices.Equal(got, []imap.UID{first}) {
		t.Fatalf("first SearchSince = %v, %v; want [%d]", got, err, first)
	}

	second := putDated(t, c, since.AddDate(0, 0, 2))
	if _, err := c.SelectForSync("INBOX"); err != nil {
		t.Fatalf("SelectForSync: %v", err)
	}
	got, err := c.SearchSince(since)
	if err != nil {
		t.Fatalf("second SearchSince: %s", err.Message)
	}
	if want := []imap.UID{first, second}; !slices.Equal(got, want) {
		t.Fatalf("second SearchSince = %v, want %v", got, want)
	}

	if n := len(wire.commands("SEARCH")); n != 1 {
		t.Errorf("sent %d dated SEARCHes; the refusal should have been remembered after the first", n)
	}
	fetches := wire.commands("FETCH")
	if len(fetches) != 2 {
		t.Fatalf("expected two INTERNALDATE reads, got %v", fetches)
	}
	if !strings.Contains(fetches[1], " "+strconv.Itoa(int(first)+1)+":*") {
		t.Errorf("the second read rescanned the folder instead of starting above UID %d: %s", first, fetches[1])
	}
}

// A server that takes the dated SEARCH keeps getting it, with no scan.
func TestSearchSinceUsesSearchWhenAccepted(t *testing.T) {
	c, wire := recordingServer(t, nil)
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	since := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	putDated(t, c, since.AddDate(0, 0, -5))
	recent := putDated(t, c, since.AddDate(0, 0, 1))
	if _, err := c.SelectForSync("INBOX"); err != nil {
		t.Fatalf("SelectForSync: %v", err)
	}
	got, err := c.SearchSince(since)
	if err != nil {
		t.Fatalf("SearchSince: %s", err.Message)
	}
	if !slices.Equal(got, []imap.UID{recent}) {
		t.Fatalf("SearchSince = %v, want [%d]", got, recent)
	}
	if c.sinceRefused.Load() {
		t.Error("an accepted SEARCH marked the server as refusing dates")
	}
	if len(wire.commands("FETCH")) != 0 {
		t.Errorf("scanned INTERNALDATE on a server that answered the SEARCH: %v", wire.commands("FETCH"))
	}
}

// Only the server refusing the command itself switches to the scan; a folder
// or account problem is reported as before.
func TestSearchRefused(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{&imap.Error{Type: imap.StatusResponseTypeNo, Text: "SEARCH: mtd: internal error: Bad token (expecting date specification)."}, true},
		{&imap.Error{Type: imap.StatusResponseTypeBad, Text: "Error in IMAP command"}, true},
		{&imap.Error{Type: imap.StatusResponseTypeNo, Code: imap.ResponseCodeParse}, true},
		{&imap.Error{Type: imap.StatusResponseTypeNo, Code: imap.ResponseCodeNonExistent}, false},
		{&imap.Error{Type: imap.StatusResponseTypeNo, Code: imap.ResponseCodeUnavailable}, false},
		{&imap.Error{Type: imap.StatusResponseTypeNo, Code: imap.ResponseCodeAuthorizationFailed}, false},
		{errNotIMAP{}, false},
	}
	for _, tc := range cases {
		if got := searchRefused(tc.err); got != tc.want {
			t.Errorf("searchRefused(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}

type errNotIMAP struct{}

func (errNotIMAP) Error() string { return "use of closed network connection" }

// A refusal with no folder selected says nothing about dates: a warmup action
// that selected a missing folder leaves the session like this, and the server
// must not be switched to the scan for good over it.
func TestSearchSinceRefusalWithoutSelectionIsNotSticky(t *testing.T) {
	c, _ := interceptingServer(t, nil, seznamRefusal)
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if _, err := c.SearchSince(time.Now()); err == nil {
		t.Fatal("a refused SEARCH with nothing selected returned no error")
	}
	if c.sinceRefused.Load() {
		t.Error("a refusal with no folder selected switched the server to the INTERNALDATE scan")
	}
}

// Without a UIDVALIDITY a recreated folder cannot be told apart, so the scan
// is not kept.
func TestSearchSinceByDateSkipsCacheWithoutUIDValidity(t *testing.T) {
	c, _ := interceptingServer(t, nil, seznamRefusal)
	if err := c.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	since := time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)
	recent := putDated(t, c, since.AddDate(0, 0, 1))
	if _, err := c.SelectForSync("INBOX"); err != nil {
		t.Fatalf("SelectForSync: %v", err)
	}
	// Stands in for a server that reports no UIDVALIDITY; recorded as the
	// sync's own view too, or the step would re-open the folder for real.
	sel := &selection{name: "INBOX", count: 1}
	c.selection.Store(sel)
	c.syncView = sel
	c.sinceRefused.Store(true)

	got, err := c.SearchSince(since)
	if err != nil {
		t.Fatalf("SearchSince: %s", err.Message)
	}
	if !slices.Equal(got, []imap.UID{recent}) {
		t.Fatalf("SearchSince = %v, want [%d]", got, recent)
	}
	if len(c.dateScans) != 0 {
		t.Errorf("cached a scan for a folder with no UIDVALIDITY: %v", c.dateScans)
	}
}
