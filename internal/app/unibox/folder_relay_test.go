package unibox

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/events"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

type fakeFolderRepo struct {
	repository.UniboxRepository
	targets []models.FolderRelayTarget
}

func (f *fakeFolderRepo) FolderRelayTargets(_ context.Context, _ uuid.UUID, _ []uuid.UUID) ([]models.FolderRelayTarget, error) {
	return f.targets, nil
}

type fakeFolderPublisher struct {
	events.Publisher
	sent   []*models.MessageFolderAction
	toWork []uuid.UUID
}

func (f *fakeFolderPublisher) PublishMessageFolder(_ context.Context, workerID uuid.UUID, action *models.MessageFolderAction) error {
	f.sent = append(f.sent, action)
	f.toWork = append(f.toWork, workerID)
	return nil
}

func folderTarget(box, worker uuid.UUID, provider, folder, providerFolder string) models.FolderRelayTarget {
	return models.FolderRelayTarget{EmailID: box, WorkerID: worker, Provider: provider, Folder: folder,
		Ref: models.MessageFolderRef{ID: uuid.New(), ProviderFolder: providerFolder}}
}

func TestFolderRelayGroupsByMailboxAndDestination(t *testing.T) {
	boxA, boxB, workerA, workerB := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	targets := []models.FolderRelayTarget{
		folderTarget(boxA, workerA, "gmail", models.FolderArchive, models.FolderInbox),
		folderTarget(boxA, workerA, "gmail", models.FolderArchive, models.FolderInbox),
		folderTarget(boxB, workerB, "smtp_imap", models.FolderArchive, models.FolderInbox),
		// A sent copy stays in an IMAP Sent folder.
		folderTarget(boxB, workerB, "smtp_imap", models.FolderArchive, models.FolderSent),
		// Already where the provider has it, and this filing did not move it.
		folderTarget(boxB, workerB, "smtp_imap", models.FolderArchive, models.FolderArchive),
	}
	filed := make([]models.FiledMessage, len(targets))
	for i, t := range targets {
		filed[i] = models.FiledMessage{ID: t.Ref.ID, Moved: true}
	}
	filed[4].Moved = false
	pub := &fakeFolderPublisher{}
	s := &uniboxService{uniboxRepository: &fakeFolderRepo{targets: targets}, publisher: pub}

	s.publishFolderRelay(context.Background(), uuid.New(), filed)

	if len(pub.sent) != 2 {
		t.Fatalf("expected one event per mailbox, got %d", len(pub.sent))
	}
	for i, act := range pub.sent {
		want, worker := 2, workerA
		if act.EmailID == boxB {
			want, worker = 1, workerB
		}
		if len(act.Messages) != want || pub.toWork[i] != worker || act.Folder != models.FolderArchive {
			t.Errorf("mailbox %v: %d messages to %v, folder %q", act.EmailID, len(act.Messages), pub.toWork[i], act.Folder)
		}
	}
}

// Undo right after Archive: the answer to the Archive has not landed, so the
// provider folder still reads inbox, and the Undo must travel anyway.
func TestFolderRelaySendsAnUndoTheProviderFolderHasNotCaughtUpWith(t *testing.T) {
	box, worker := uuid.New(), uuid.New()
	target := folderTarget(box, worker, "smtp_imap", models.FolderInbox, models.FolderInbox)
	pub := &fakeFolderPublisher{}
	s := &uniboxService{uniboxRepository: &fakeFolderRepo{targets: []models.FolderRelayTarget{target}}, publisher: pub}

	s.publishFolderRelay(context.Background(), uuid.New(), []models.FiledMessage{{ID: target.Ref.ID, Moved: true}})

	if len(pub.sent) != 1 {
		t.Fatalf("the undo was not relayed")
	}
}

func TestFolderRelayChunks(t *testing.T) {
	box, worker := uuid.New(), uuid.New()
	n := models.FolderRelayChunk + 10
	targets := make([]models.FolderRelayTarget, n)
	filed := make([]models.FiledMessage, n)
	for i := range targets {
		targets[i] = folderTarget(box, worker, "gmail", models.FolderTrash, models.FolderInbox)
		filed[i] = models.FiledMessage{ID: targets[i].Ref.ID, Moved: true}
	}
	pub := &fakeFolderPublisher{}
	s := &uniboxService{uniboxRepository: &fakeFolderRepo{targets: targets}, publisher: pub}

	s.publishFolderRelay(context.Background(), uuid.New(), filed)

	if len(pub.sent) != 2 || len(pub.sent[0].Messages) != models.FolderRelayChunk || len(pub.sent[1].Messages) != 10 {
		t.Fatalf("chunks: %d", len(pub.sent))
	}
}

// The reader files by thread and by id at once, so the second statement finds
// the row already moved; the first report is the one that counts.
func TestFolderRelayKeepsTheMoveWhenARowComesBackTwice(t *testing.T) {
	box, worker := uuid.New(), uuid.New()
	target := folderTarget(box, worker, "outlook", models.FolderInbox, models.FolderInbox)
	pub := &fakeFolderPublisher{}
	s := &uniboxService{uniboxRepository: &fakeFolderRepo{targets: []models.FolderRelayTarget{target}}, publisher: pub}

	s.publishFolderRelay(context.Background(), uuid.New(), []models.FiledMessage{
		{ID: target.Ref.ID, Moved: true},
		{ID: target.Ref.ID, Moved: false},
	})

	if len(pub.sent) != 1 {
		t.Fatalf("the undo was dropped when the row came back twice")
	}
}
