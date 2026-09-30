package worker

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/app/worker/wmail"
	"github.com/warmbly/warmbly/internal/models"
)

// fakeFolderConn holds each folder's messages by Message-ID and UID, and
// moves them the way a server with COPYUID does when copyUID is set.
type fakeFolderConn struct {
	wmail.ImapConn
	folders map[string]map[string]uint32
	next    uint32
	copyUID bool
	moves   []string
}

func (f *fakeFolderConn) FindUIDsByMessageIDs(_ context.Context, folder string, ids []string) (map[string]uint32, error) {
	found := map[string]uint32{}
	for _, id := range ids {
		if uid, ok := f.folders[folder][id]; ok {
			found[id] = uid
		}
	}
	return found, nil
}

func (f *fakeFolderConn) MoveUIDs(_ context.Context, src, dst string, uids []uint32) (map[uint32]uint32, uint32, error) {
	f.moves = append(f.moves, fmt.Sprintf("%s>%s %v", src, dst, uids))
	moved := map[uint32]uint32{}
	if f.folders[dst] == nil {
		f.folders[dst] = map[string]uint32{}
	}
	for id, uid := range f.folders[src] {
		if !slices.Contains(uids, uid) {
			continue
		}
		f.next++
		delete(f.folders[src], id)
		f.folders[dst][id] = f.next
		if f.copyUID {
			moved[uid] = f.next
		}
	}
	if f.copyUID {
		return moved, 7, nil
	}
	return moved, 0, nil
}

func folderRelayMail(conn *fakeFolderConn) *wmail.WMail {
	return &wmail.WMail{SmtpImapData: &wmail.SmtpImapData{
		ImapClient: conn,
		Mailboxes: []*models.Mailbox{
			{Name: "INBOX", UIDValidity: 1},
			{Name: "Archive", Attrs: []string{"\\Archive"}, UIDValidity: 7},
			{Name: "Trash", Attrs: []string{"\\Trash"}, UIDValidity: 9},
		},
	}}
}

func TestImapFolderRelayMovesAndReportsTheNewLocation(t *testing.T) {
	for _, copyUID := range []bool{true, false} {
		t.Run(fmt.Sprintf("copyuid=%v", copyUID), func(t *testing.T) {
			conn := &fakeFolderConn{copyUID: copyUID, next: 100, folders: map[string]map[string]uint32{
				"INBOX": {"<a@x>": 5, "<b@x>": 6},
			}}
			a, b := uuid.New(), uuid.New()
			action := models.MessageFolderAction{EmailID: uuid.New(), Folder: models.FolderArchive, Messages: []models.MessageFolderRef{
				{ID: a, UID: 5, FolderPath: "INBOX", RFCMessageID: "<a@x>"},
				{ID: b, UID: 6, FolderPath: "INBOX", RFCMessageID: "<b@x>"},
			}}

			got := (&WorkerService{}).relayFolderImap(context.Background(), folderRelayMail(conn), action)

			if len(conn.moves) != 1 {
				t.Fatalf("one folder's share should be one MOVE, got %v", conn.moves)
			}
			if len(got) != 2 {
				t.Fatalf("expected an answer per moved message, got %d", len(got))
			}
			for _, u := range got {
				if u.FolderPath != "Archive" || u.UID == 0 || u.Mailbox != 7 {
					t.Errorf("answer for %v lacks the new location: %+v", u.ID, u)
				}
			}
		})
	}
}

// Undo right after Archive: the store still names the inbox UID the first
// relay already moved, so the message is found by Message-ID instead.
func TestImapFolderRelayFindsAMessageTheStoreHasStale(t *testing.T) {
	conn := &fakeFolderConn{copyUID: true, next: 200, folders: map[string]map[string]uint32{
		"INBOX":   {},
		"Archive": {"<a@x>": 150},
	}}
	id := uuid.New()
	action := models.MessageFolderAction{EmailID: uuid.New(), Folder: models.FolderInbox, Messages: []models.MessageFolderRef{
		{ID: id, UID: 5, FolderPath: "INBOX", RFCMessageID: "<a@x>"},
	}}

	got := (&WorkerService{}).relayFolderImap(context.Background(), folderRelayMail(conn), action)

	if want := []string{"Archive>INBOX [150]"}; !slices.Equal(conn.moves, want) {
		t.Fatalf("moves = %v, want %v", conn.moves, want)
	}
	if len(got) != 1 || got[0].ID != id || got[0].FolderPath != "INBOX" {
		t.Fatalf("answers = %+v", got)
	}
}

func TestImapFolderRelayLeavesWhatItCannotFindOrKey(t *testing.T) {
	conn := &fakeFolderConn{folders: map[string]map[string]uint32{"INBOX": {"<a@x>": 5}}}
	action := models.MessageFolderAction{EmailID: uuid.New(), Folder: models.FolderTrash, Messages: []models.MessageFolderRef{
		// Keyed by folder and UID: a moved copy would come back as a new message.
		{ID: uuid.New(), UID: 9, FolderPath: "INBOX", RFCMessageID: "no-msgid/INBOX/1/9"},
		// Gone from every folder: nothing to move and nothing to report.
		{ID: uuid.New(), UID: 8, FolderPath: "INBOX", RFCMessageID: "<gone@x>"},
	}}

	got := (&WorkerService{}).relayFolderImap(context.Background(), folderRelayMail(conn), action)

	if len(conn.moves) != 0 || len(got) != 0 {
		t.Fatalf("moves = %v answers = %+v", conn.moves, got)
	}
}

func TestImapFolderRelayAnswersAMessageAlreadyThere(t *testing.T) {
	conn := &fakeFolderConn{folders: map[string]map[string]uint32{"Trash": {"<a@x>": 3}}}
	id := uuid.New()
	action := models.MessageFolderAction{EmailID: uuid.New(), Folder: models.FolderTrash, Messages: []models.MessageFolderRef{
		{ID: id, UID: 5, FolderPath: "INBOX", RFCMessageID: "<a@x>"},
	}}

	got := (&WorkerService{}).relayFolderImap(context.Background(), folderRelayMail(conn), action)

	if len(conn.moves) != 0 {
		t.Fatalf("a message already in the trash must not be moved: %v", conn.moves)
	}
	if len(got) != 1 || got[0].UID != 3 || got[0].Mailbox != 9 {
		t.Fatalf("answers = %+v", got)
	}
}
