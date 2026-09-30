package goog

import (
	"slices"
	"testing"

	"github.com/warmbly/warmbly/internal/models"
)

// Each destination has to land on the folder Folder() reads back from the
// labels, or the sync would report the relayed move as the provider's own.
func TestFolderLabelsRoundTripThroughFolder(t *testing.T) {
	starts := [][]string{{Inbox}, {Inbox, Unread}, {Trash}, {Spam}, {}, {Inbox, Trash}}
	for _, folder := range []string{models.FolderInbox, models.FolderArchive, models.FolderTrash} {
		add, remove, ok := FolderLabels(folder)
		if !ok {
			t.Fatalf("%s is filable", folder)
		}
		for _, start := range starts {
			labels := slices.DeleteFunc(slices.Clone(start), func(l string) bool { return slices.Contains(remove, l) })
			for _, l := range add {
				if !slices.Contains(labels, l) {
					labels = append(labels, l)
				}
			}
			if got := Folder(labels); got != folder {
				t.Errorf("%s from %v: labels %v read back as %s", folder, start, labels, got)
			}
		}
	}
	if _, _, ok := FolderLabels(models.FolderSpam); ok {
		t.Error("spam is a verdict, not a filing")
	}
}
