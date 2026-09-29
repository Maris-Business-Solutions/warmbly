package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/warmbly/warmbly/internal/models"
)

// folderHolder answers which folders hold the message, and fails on one.
type folderHolder struct {
	holds  map[string]bool
	fails  string
	opened []string
}

func (h *folderHolder) HoldsMessageID(_ context.Context, mailbox, _ string) (bool, error) {
	h.opened = append(h.opened, mailbox)
	if mailbox == h.fails {
		return false, errors.New("cannot select")
	}
	return h.holds[mailbox], nil
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
		{"nowhere synced is inconclusive", nil, "", "", false},
		{"a folder that cannot be opened is an error", nil, "Warmbly", "", true},
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
			for _, name := range h.opened {
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
