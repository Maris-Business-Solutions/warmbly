package jobs

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/models"
)

// A folder the provider reports is resolved against where the provider last
// had the message, so Gmail's own moves land and Warmbly's filing survives.
func TestFolderUpdateFollowsOnlyProviderMoves(t *testing.T) {
	for _, tc := range []struct {
		name           string
		folder         string
		providerFolder string
		reported       string
		wantFolder     string // "" means nothing written
	}{
		{"archived in Gmail", models.FolderInbox, models.FolderInbox, models.FolderArchive, models.FolderArchive},
		{"trashed in Gmail", models.FolderInbox, models.FolderInbox, models.FolderTrash, models.FolderTrash},
		{"moved back to the inbox in Gmail", models.FolderArchive, models.FolderArchive, models.FolderInbox, models.FolderInbox},
		{"filed in Warmbly, still in the Gmail inbox", models.FolderArchive, models.FolderInbox, models.FolderInbox, ""},
		{"unknown folder", models.FolderInbox, models.FolderInbox, "elsewhere", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, repo := seenSyncService(&models.EmailMessageStoreData{Folder: tc.folder, ProviderFolder: tc.providerFolder})
			if err := s.HandleFolderUpdate(context.Background(), &models.JobEventFolderUpdate{
				UserID: uuid.New(), EmailID: uuid.New(), ID: uuid.New(),
				Folder: tc.reported,
			}); err != nil {
				t.Fatal(err)
			}
			if tc.wantFolder == "" {
				if len(repo.updates) != 0 {
					t.Fatalf("wrote %+v, want nothing", repo.updates)
				}
				return
			}
			if len(repo.updates) != 1 {
				t.Fatalf("wrote %d updates, want 1", len(repo.updates))
			}
			u := repo.updates[0]
			if u.ProviderFolder == nil || *u.ProviderFolder != tc.reported {
				t.Errorf("provider_folder = %v, want %q", u.ProviderFolder, tc.reported)
			}
			if u.Folder == nil || *u.Folder != tc.wantFolder {
				t.Errorf("folder = %v, want %q", u.Folder, tc.wantFolder)
			}
		})
	}
}
