package wmail

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

// labelGmail is the Gmail API as the folder paths see it: an inbox listing
// and a minimal messages.get that answers with labels, or 404 when a message
// is not in labels at all.
type labelGmail struct {
	inbox   []string
	labels  map[string][]string
	queries []string
	gets    []string
}

func (g *labelGmail) serve(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if id, isGet := strings.CutPrefix(r.URL.Path, "/gmail/v1/users/me/messages/"); isGet {
			g.gets = append(g.gets, id)
			labels, ok := g.labels[id]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error":{"code":404,"message":"Requested entity was not found."}}`))
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "labelIds": labels})
			return
		}
		g.queries = append(g.queries, r.URL.Query().Get("q"))
		msgs := make([]map[string]string, 0, len(g.inbox))
		for _, id := range g.inbox {
			msgs = append(msgs, map[string]string{"id": id, "threadId": "t-" + id})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"messages": msgs})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func folderUpdates(events []captured) map[uuid.UUID]string {
	out := map[uuid.UUID]string{}
	for _, e := range events {
		if e.eventType != models.JobEventTypeFolderUpdate {
			continue
		}
		u := e.body.(*models.JobEventFolderUpdate)
		out[u.ID] = u.Folder
	}
	return out
}

// Archiving in Gmail only removes the INBOX label, and that has to reach the
// platform as a move to archive, not only as a flag nobody files by.
func TestGmailLabelChangeReportsTheFolder(t *testing.T) {
	g := &labelGmail{labels: map[string][]string{
		"gone-to-trash": {"TRASH", "UNREAD"},
		"stale-record":  {"CATEGORY_UPDATES"},
	}}
	var events []captured
	w := newGoogleTestMail(t, g.serve(t), &events)
	rowID := uuid.New()
	w.EmailMessageMapRepository = knownMessageMap{id: rowID.String()}

	cases := []struct {
		name    string
		id      string
		changed []string
		current []string
		added   bool
		want    string
	}{
		{"archive", "m1", []string{"INBOX"}, []string{"CATEGORY_UPDATES"}, false, models.FolderArchive},
		{"move to inbox", "m2", []string{"INBOX"}, []string{"INBOX", "UNREAD"}, true, models.FolderInbox},
		{"report spam", "m3", []string{"SPAM"}, []string{"SPAM"}, true, models.FolderSpam},
		{"labels looked up when the record has none", "gone-to-trash", []string{"TRASH"}, nil, true, models.FolderTrash},
		{"labels that contradict the change are looked up", "stale-record", []string{"INBOX"}, []string{"INBOX"}, false, models.FolderArchive},
		{"a star moves nothing", "m4", []string{"STARRED"}, []string{"INBOX", "STARRED"}, true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			events = nil
			if err := w.emitGoogleLabelEvents(t.Context(), tc.id, tc.changed, tc.current, tc.added); err != nil {
				t.Fatalf("emit: %v", err)
			}
			got, moved := folderUpdates(events)[rowID]
			if tc.want == "" {
				if moved {
					t.Fatalf("reported a move to %q for a label that is not a folder", got)
				}
				return
			}
			if got != tc.want {
				t.Fatalf("folder = %q, want %q", got, tc.want)
			}
		})
	}
	if strings.Join(g.gets, ",") != "gone-to-trash,stale-record" {
		t.Errorf("looked up %v, want only the records whose labels could not be trusted", g.gets)
	}
}

// providerRows stands in for the control plane's provider-folder listing.
type providerRows struct {
	fakeSyncContext
	rows  []repository.ProviderFolderMessage
	calls int
}

func (p *providerRows) ListProviderFolderMessages(context.Context, uuid.UUID, uuid.UUID, []string, int) ([]repository.ProviderFolderMessage, error) {
	p.calls++
	return p.rows, nil
}

// Mail archived in Gmail before labels were followed stays in the unibox
// inbox until the reconciliation finds it.
func TestGmailReconcileMovesWhatGmailMoved(t *testing.T) {
	now := time.Now()
	row := func(providerID, folder string, age time.Duration) repository.ProviderFolderMessage {
		return repository.ProviderFolderMessage{ID: uuid.New(), ProviderID: providerID, ProviderFolder: folder, InternalDate: now.Add(-age)}
	}
	stillInbox := row("still-inbox", models.FolderInbox, time.Hour)
	archived := row("archived", models.FolderInbox, 2*time.Hour)
	trashed := row("trashed", models.FolderInbox, 3*time.Hour)
	deleted := row("deleted", models.FolderInbox, 4*time.Hour)
	backToInbox := row("back", models.FolderArchive, 5*time.Hour)
	stillArchived := row("old-archive", models.FolderArchive, 6*time.Hour)

	g := &labelGmail{
		inbox: []string{"still-inbox", "back"},
		labels: map[string][]string{
			"archived": {"CATEGORY_PERSONAL"},
			"trashed":  {"TRASH"},
		},
	}
	var events []captured
	w := newGoogleTestMail(t, g.serve(t), &events)
	sc := &providerRows{rows: []repository.ProviderFolderMessage{stillInbox, archived, trashed, deleted, backToInbox, stillArchived}}
	w.SyncContext = sc

	if merr := w.googleReconcileFolders(t.Context(), now, &tickStats{}); merr != nil {
		t.Fatalf("reconcile: %v", merr.Message)
	}

	want := map[uuid.UUID]string{
		archived.ID:    models.FolderArchive,
		trashed.ID:     models.FolderTrash,
		backToInbox.ID: models.FolderInbox,
	}
	got := folderUpdates(events)
	if len(got) != len(want) {
		t.Fatalf("reported %d moves, want %d: %v", len(got), len(want), got)
	}
	for id, folder := range want {
		if got[id] != folder {
			t.Errorf("row %s moved to %q, want %q", id, got[id], folder)
		}
	}
	// Only inbox rows missing from the listing cost a lookup.
	if strings.Join(g.gets, ",") != "archived,trashed,deleted" {
		t.Errorf("looked up %v, want archived, trashed, deleted", g.gets)
	}
	if len(g.queries) != 1 || !strings.HasPrefix(g.queries[0], "in:inbox after:") {
		t.Errorf("inbox listing queries = %v", g.queries)
	}

	// Inside the interval nothing runs; after it, a message already found
	// gone is not looked up again.
	events, g.gets = nil, nil
	if merr := w.googleReconcileFolders(t.Context(), now.Add(time.Minute), &tickStats{}); merr != nil {
		t.Fatalf("second pass: %v", merr.Message)
	}
	if sc.calls != 1 {
		t.Errorf("listed stored rows %d times inside the interval, want 1", sc.calls)
	}
	sc.rows = []repository.ProviderFolderMessage{stillInbox, deleted}
	if merr := w.googleReconcileFolders(t.Context(), now.Add(7*time.Hour), &tickStats{}); merr != nil {
		t.Fatalf("third pass: %v", merr.Message)
	}
	if len(g.gets) != 0 {
		t.Errorf("looked up %v again, want nothing", g.gets)
	}
	if len(events) != 0 {
		t.Errorf("reported %d events for rows already where Gmail has them", len(events))
	}

	// A day on, the same message is worth one more look.
	if merr := w.googleReconcileFolders(t.Context(), now.Add(25*time.Hour), &tickStats{}); merr != nil {
		t.Fatalf("fourth pass: %v", merr.Message)
	}
	if strings.Join(g.gets, ",") != "deleted" {
		t.Errorf("looked up %v a day later, want deleted", g.gets)
	}
}
