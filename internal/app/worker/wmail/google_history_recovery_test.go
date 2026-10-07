package wmail

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

type recoveryMessageMap struct {
	data map[string]repository.EmailMessageData
}

func (m *recoveryMessageMap) Add(_ context.Context, data repository.EmailMessageData) error {
	m.data[data.MessageID] = data
	return nil
}
func (m *recoveryMessageMap) Get(_ context.Context, _, _ uuid.UUID, id string) (*repository.EmailMessageData, error) {
	data, ok := m.data[id]
	if !ok {
		return nil, nil
	}
	return &data, nil
}
func (m *recoveryMessageMap) Del(_ context.Context, _, _ uuid.UUID, id string, _ uuid.UUID) error {
	delete(m.data, id)
	return nil
}

type recoveryRows struct {
	fakeSyncContext
	rows   []repository.ProviderFolderMessage
	err    error
	afters []string
}

func (r *recoveryRows) ListProviderMessages(_ context.Context, _, _ uuid.UUID, after *uuid.UUID, limit int) ([]repository.ProviderFolderMessage, error) {
	if r.err != nil {
		return nil, r.err
	}
	var out []repository.ProviderFolderMessage
	for _, m := range r.rows {
		if after == nil || m.ID.String() > after.String() {
			out = append(out, m)
		}
	}
	if after != nil {
		r.afters = append(r.afters, after.String())
	}
	return out[:min(len(out), limit)], nil
}

type recoveryGmail struct {
	tokens       []string
	historyIDs   []string
	includesSpam bool
	refuseStored bool
}

func (g *recoveryGmail) serve(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/history"):
			id := r.URL.Query().Get("startHistoryId")
			g.historyIDs = append(g.historyIDs, id)
			if id == "10" {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error":{"code":404,"message":"expired"}}`))
				return
			}
			_, _ = w.Write([]byte(`{"historyId":"101","history":[{"id":"101","messagesAdded":[{"message":{"id":"during-recovery","threadId":"thread-live"}}]}]}`))
		case strings.HasSuffix(r.URL.Path, "/profile"):
			_, _ = w.Write([]byte(`{"historyId":"100"}`))
		case strings.HasSuffix(r.URL.Path, "/messages"):
			token := r.URL.Query().Get("pageToken")
			g.tokens = append(g.tokens, token)
			g.includesSpam = r.URL.Query().Get("includeSpamTrash") == "true"
			if token == "expired-page" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":{"code":400,"message":"Invalid pageToken"}}`))
				return
			}
			if !strings.Contains(r.URL.Query().Get("q"), "after:") {
				t.Error("recovery must be bounded by its original import window")
			}
			if token == "" {
				_, _ = w.Write([]byte(`{"messages":[{"id":"missed-1"},{"id":"missed-2"}],"nextPageToken":"page-2"}`))
				return
			}
			_, _ = w.Write([]byte(`{"messages":[{"id":"missed-3"}]}`))
		case strings.HasSuffix(r.URL.Path, "/messages/deleted"):
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":404,"message":"deleted"}}`))
		case strings.HasSuffix(r.URL.Path, "/messages/tracked") && g.refuseStored:
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"code":429,"message":"quota"}}`))
		default:
			id := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
			_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "threadId": "thread-" + id, "labelIds": []string{"TRASH", "UNREAD", "Label_current"}, "payload": map[string]any{"headers": []map[string]string{{"name": "Message-Id", "value": "<" + id + "@gmail.test>"}}}})
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newRecoveryMail(t *testing.T, g *recoveryGmail, events *[]captured) *WMail {
	t.Helper()
	w := newGoogleTestMail(t, g.serve(t), events)
	w.GoogleData.LastHistoryID = 10
	w.EmailMessageMapRepository = &recoveryMessageMap{data: map[string]repository.EmailMessageData{}}
	w.SyncContext = &recoveryRows{rows: []repository.ProviderFolderMessage{
		{ID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), ProviderID: "tracked", ProviderFolder: models.FolderInbox, Flags: []string{"Label_old", "INBOX", "Auto-Submitted:auto-generated"}},
		{ID: uuid.MustParse("22222222-2222-2222-2222-222222222222"), ProviderID: "deleted"},
	}}
	w.tracker.state.BackfillStatus = models.SyncBackfillComplete
	return w
}

func TestGoogleExpiredHistoryRecoveryResumesAndReplaysConcurrentChanges(t *testing.T) {
	g := &recoveryGmail{}
	var events []captured
	w := newRecoveryMail(t, g, &events)
	budget := &fixedBudget{allow: 1}
	w.gov = budget
	if err := w.SyncGoogle(t.Context()); err != nil {
		t.Fatal(err)
	}
	recovery := w.tracker.state.BackfillCursor.GoogleRecovery
	if recovery == nil || recovery.HistoryID != 100 || recovery.PageToken != "" || w.GoogleData.LastHistoryID != 10 {
		t.Fatalf("premature checkpoint: recovery=%+v history=%d", recovery, w.GoogleData.LastHistoryID)
	}
	if budget.observed != 0 {
		t.Fatal("recovered messages counted as an inbound flood")
	}

	// A worker reload only has the durable cursor and message map, not local progress.
	encoded, err := json.Marshal(w.tracker.state)
	if err != nil {
		t.Fatal(err)
	}
	var seed models.SyncState
	if err := json.Unmarshal(encoded, &seed); err != nil {
		t.Fatal(err)
	}
	w.tracker = newSyncTracker(&seed, func(models.SyncState) error { return nil })
	budget.allow = 10
	if err := w.SyncGoogle(t.Context()); err != nil {
		t.Fatal(err)
	}
	if w.tracker.state.BackfillCursor.GoogleRecovery != nil || w.GoogleData.LastHistoryID != 100 {
		t.Fatalf("recovery not complete: %+v", w.tracker.state)
	}
	if strings.Join(g.tokens, ",") != ",,page-2" || !g.includesSpam {
		t.Fatalf("recovery listing tokens=%v includesSpam=%t", g.tokens, g.includesSpam)
	}
	if len(newEmails(events)) != 3 {
		t.Fatalf("recovery emitted %d arrivals, want 3 without duplicates", len(newEmails(events)))
	}
	if folderUpdates(events)[uuid.MustParse("11111111-1111-1111-1111-111111111111")] != models.FolderTrash || len(removedIDs(events)) != 1 {
		t.Fatal("stored state was not reconciled")
	}
	var removed []string
	for _, e := range events {
		if e.eventType == models.JobEventTypeFlagsRemove {
			removed = append(removed, e.body.(*models.JobEventFlags).Flags...)
		}
	}
	if !slices.Contains(removed, "Label_old") || !slices.Contains(removed, "INBOX") || !slices.Contains(removed, "\\Seen") || slices.Contains(removed, "Auto-Submitted:auto-generated") {
		t.Fatalf("removed flags=%v", removed)
	}
	if err := w.SyncGoogle(t.Context()); err != nil {
		t.Fatal(err)
	}
	if w.GoogleData.LastHistoryID != 101 || strings.Join(g.historyIDs, ",") != "10,100" || len(newEmails(events)) != 4 {
		t.Fatalf("changes during recovery lost: ids=%v arrivals=%d checkpoint=%d", g.historyIDs, len(newEmails(events)), w.GoogleData.LastHistoryID)
	}
}

func TestGoogleHistoryRecoveryHoldsCheckpointOnStoredLookupAndPublicationFailure(t *testing.T) {
	g := &recoveryGmail{refuseStored: true}
	var events []captured
	w := newRecoveryMail(t, g, &events)
	_ = w.SyncGoogle(t.Context())
	recovery := w.tracker.state.BackfillCursor.GoogleRecovery
	if recovery == nil || !recovery.MessagesDone || recovery.StoredAfter != "" || w.GoogleData.LastHistoryID != 10 {
		t.Fatalf("lookup failure advanced recovery: %+v", recovery)
	}
	g.refuseStored = false
	callback := w.onEvent
	w.onEvent = func(kind models.JobEventType, body any) error {
		if kind == models.JobEventTypeHistoryIDUpdate {
			return errors.New("relay unavailable")
		}
		return callback(kind, body)
	}
	_ = w.SyncGoogle(t.Context())
	if w.GoogleData.LastHistoryID != 10 || w.tracker.state.BackfillCursor.GoogleRecovery == nil {
		t.Fatal("failed baseline publication cleared recovery")
	}
	w.onEvent = callback
	if err := w.SyncGoogle(t.Context()); err != nil {
		t.Fatal(err)
	}
	if w.GoogleData.LastHistoryID != 100 || w.tracker.state.BackfillCursor.GoogleRecovery != nil {
		t.Fatal("baseline publication was not retried")
	}
	if len(g.tokens) != 2 {
		t.Fatalf("completed message pages were restarted: %v", g.tokens)
	}
}

func TestGoogleHistoryRecoveryRestartsAnExpiredPageWithoutReplacingBaseline(t *testing.T) {
	g := &recoveryGmail{}
	var events []captured
	w := newRecoveryMail(t, g, &events)
	w.gov = &fixedBudget{allow: 0}
	if err := w.SyncGoogle(t.Context()); err != nil {
		t.Fatal(err)
	}
	recovery := w.tracker.state.BackfillCursor.GoogleRecovery
	recovery.PageToken = "expired-page"
	since := recovery.Since
	if err := w.SyncGoogle(t.Context()); err != nil {
		t.Fatal(err)
	}
	if recovery.PageToken != "" || !recovery.Since.Equal(since) || recovery.HistoryID != 100 || w.GoogleData.LastHistoryID != 10 {
		t.Fatalf("page restart changed recovery scope: %+v", recovery)
	}
	w.gov = &fixedBudget{allow: 10}
	if err := w.SyncGoogle(t.Context()); err != nil {
		t.Fatal(err)
	}
	if w.GoogleData.LastHistoryID != 100 || len(newEmails(events)) != 3 {
		t.Fatal("expired-page recovery did not finish")
	}
}

func newEmails(events []captured) []*models.JobEventNewEmail {
	var out []*models.JobEventNewEmail
	for _, e := range events {
		if e.eventType == models.JobEventTypeNewEmail {
			out = append(out, e.body.(*models.JobEventNewEmail))
		}
	}
	return out
}
