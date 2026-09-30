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

// countingReads records reads: the unibox's GetByID marks a message seen.
type countingReads struct {
	*seenSyncRepo
	reads int
}

func (r *countingReads) GetByID(ctx context.Context, userID, id uuid.UUID) (*models.EmailMessageStoreData, error) {
	r.reads++
	return r.seenSyncRepo.GetByID(ctx, userID, id)
}

// A relayed filing's answer records the provider's placement and the new
// handles, and never the folder: the filing may have been undone meanwhile.
// It must not read the row either, or archiving unread mail marks it read.
func TestRelayedFolderUpdateLeavesTheFolderAlone(t *testing.T) {
	s, base := seenSyncService(&models.EmailMessageStoreData{
		Folder: models.FolderInbox, ProviderFolder: models.FolderInbox, GmailID: "old",
	})
	repo := &countingReads{seenSyncRepo: base}
	s.UniboxRepository = repo
	if err := s.HandleFolderUpdate(context.Background(), &models.JobEventFolderUpdate{
		UserID: uuid.New(), EmailID: uuid.New(), ID: uuid.New(),
		Folder: models.FolderArchive, Relayed: true, ProviderID: "new",
		FolderPath: "Archive", UID: 12, Mailbox: 7,
	}); err != nil {
		t.Fatal(err)
	}
	if len(repo.updates) != 1 {
		t.Fatalf("wrote %d updates, want 1", len(repo.updates))
	}
	u := repo.updates[0]
	if u.Folder != nil {
		t.Errorf("folder written to %q", *u.Folder)
	}
	if u.ProviderFolder == nil || *u.ProviderFolder != models.FolderArchive {
		t.Errorf("provider_folder = %v", u.ProviderFolder)
	}
	if u.ProviderID == nil || *u.ProviderID != "new" || u.UID == nil || *u.UID != 12 || u.FolderPath == nil || *u.FolderPath != "Archive" {
		t.Errorf("handles not moved: %+v", u)
	}
	if u.Seen != nil || repo.reads != 0 {
		t.Errorf("the answer touched read state: seen=%v reads=%d", u.Seen, repo.reads)
	}

	// The sync then reports the same placement, which is no provider move.
	s, base = seenSyncService(&models.EmailMessageStoreData{Folder: models.FolderInbox, ProviderFolder: models.FolderArchive})
	if err := s.HandleFolderUpdate(context.Background(), &models.JobEventFolderUpdate{
		UserID: uuid.New(), EmailID: uuid.New(), ID: uuid.New(), Folder: models.FolderArchive,
	}); err != nil {
		t.Fatal(err)
	}
	if len(base.updates) != 0 {
		t.Fatalf("the relayed move's echo refiled the message: %+v", base.updates)
	}
}
