package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/warmbly/warmbly/internal/models"
)

// folderHolder answers which folders hold the message, and which it cannot open.
type folderHolder struct {
	holds    map[string]bool
	fails    string
	searched []string
}

func (h *folderHolder) LocateMessageID(_ context.Context, mailboxes []string, _ string) ([]string, int, error) {
	h.searched = mailboxes
	var held []string
	unsearched := 0
	for _, m := range mailboxes {
		if m == h.fails {
			unsearched++
			continue
		}
		if h.holds[m] {
			held = append(held, m)
		}
	}
	return held, unsearched, nil
}

func TestLocateImapMessageReportsWhereTheMessageIs(t *testing.T) {
	boxes := []*models.Mailbox{
		{Name: "INBOX"},
		{Name: "Trash", Attrs: []string{"\\Trash"}},
		{Name: "[Parent]", Attrs: []string{"\\Noselect"}},
		{Name: "Warmbly"},
	}
	cases := []struct {
		name  string
		holds map[string]bool
		fails string
		want  string
		err   bool
	}{
		{"moved to another folder", map[string]bool{"Warmbly": true}, "", models.WarmupRemovalPresent, false},
		{"a copy outside the trash wins", map[string]bool{"Trash": true, "INBOX": true}, "", models.WarmupRemovalPresent, false},
		{"only in the trash", map[string]bool{"Trash": true}, "", models.WarmupRemovalTrashed, false},
		{"only in the trash with a folder unsearched is inconclusive", map[string]bool{"Trash": true}, "Warmbly", "", false},
		{"nowhere synced is inconclusive", nil, "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := &folderHolder{holds: tc.holds, fails: tc.fails}
			got, err := locateImapMessage(context.Background(), h, boxes, "<m@example.test>")
			if (err != nil) != tc.err {
				t.Fatalf("err = %v, want error %v", err, tc.err)
			}
			if got != tc.want {
				t.Fatalf("outcome = %q, want %q", got, tc.want)
			}
			for _, name := range h.searched {
				if name == "[Parent]" {
					t.Fatal("searched a folder listed only as hierarchy")
				}
			}
		})
	}
}

func TestRemovalOutcome(t *testing.T) {
	cases := []struct {
		found, trashed bool
		want           string
	}{
		{true, false, models.WarmupRemovalPresent},
		{true, true, models.WarmupRemovalTrashed},
		{false, false, models.WarmupRemovalGone},
	}
	for _, tc := range cases {
		if got, err := removalOutcome(tc.found, tc.trashed, nil); err != nil || got != tc.want {
			t.Fatalf("removalOutcome(%v, %v) = %q, %v; want %q", tc.found, tc.trashed, got, err, tc.want)
		}
	}
	if got, err := removalOutcome(false, false, errors.New("down")); err == nil || got != "" {
		t.Fatalf("a failed search answered %q, %v", got, err)
	}
}
