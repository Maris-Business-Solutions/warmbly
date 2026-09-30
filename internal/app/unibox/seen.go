package unibox

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/observability/errs"
)

func (s *uniboxService) MarkSeen(ctx context.Context, userID, emailID uuid.UUID, seen bool) *errx.Error {
	if err := s.uniboxRepository.MarkSeen(ctx, userID, emailID, seen); err != nil {
		errs.CaptureException(err)
		return errx.InternalError()
	}

	return nil
}

func (s *uniboxService) MarkSeenBulk(ctx context.Context, orgID uuid.UUID, data *models.MarkSeen) (*models.MarkSeen, *errx.Error) {
	if len(data.EmailIDs) > 500 || len(data.ThreadIDs) > 500 {
		return nil, errx.ErrSeenMax
	}

	// A folder sweep and an id list are different requests; refuse the
	// ambiguous combination instead of guessing which one was meant.
	if data.Folder != "" {
		if len(data.EmailIDs) > 0 || len(data.ThreadIDs) > 0 {
			return nil, errx.ErrSeenFolderAndIDs
		}
		if !models.ValidFolder(data.Folder) {
			return nil, errx.ErrUniboxFolder
		}
		changed, err := s.uniboxRepository.MarkSeenByFolder(ctx, orgID, data.Folder, data.Seen)
		if err != nil {
			errs.CaptureException(err)
			return nil, errx.InternalError()
		}
		s.relaySeen(ctx, orgID, changed)
		return data, nil
	}

	// Conversations and ids can arrive together: the list marks a row read by
	// thread, the reader marks the messages it has open by id.
	var changed []uuid.UUID
	if len(data.ThreadIDs) > 0 {
		byThread, err := s.uniboxRepository.MarkSeenByThreads(ctx, orgID, data.ThreadIDs, data.Seen)
		if err != nil {
			errs.CaptureException(err)
			return nil, errx.InternalError()
		}
		changed = append(changed, byThread...)
	}
	if len(data.EmailIDs) > 0 {
		byID, err := s.uniboxRepository.MarkSeenBulk(ctx, orgID, data.EmailIDs, data.Seen)
		if err != nil {
			errs.CaptureException(err)
			return nil, errx.InternalError()
		}
		changed = append(changed, byID...)
	}
	s.relaySeen(ctx, orgID, changed)

	return data, nil
}

// relaySeen carries a read/unread change out to the mailboxes themselves, so
// a thread read in Warmbly is read in Gmail too.
//
// Detached and best-effort. The store is the customer's view and has already
// been written, so this must not hold the response open behind a slow broker,
// and a worker that cannot be reached must not fail the request. Nothing
// retries: the provider's own state is what the next sync brings back anyway.
//
// What is relayed is the state the ROW now holds, read back inside the
// lookup, not the state the request asked for. Two people toggling the same
// conversation in opposite directions at the same moment can still race, but
// the loser then relays the winner's answer rather than its own.
func (s *uniboxService) relaySeen(ctx context.Context, orgID uuid.UUID, changed []uuid.UUID) {
	if s.publisher == nil || len(changed) == 0 {
		return
	}

	go func() {
		// Detached from the request, bounded so a wedged broker cannot leak a
		// goroutine per press.
		bg, cancel := context.WithTimeout(context.WithoutCancel(ctx), seenRelayTimeout)
		defer cancel()
		s.publishSeenRelay(bg, orgID, changed)
	}()
}

// seenRelayTimeout bounds one relay. Generous, because "mark all as read" on
// a busy folder is many events, and it exists to end a wedged publish rather
// than to pace a healthy one.
const seenRelayTimeout = 2 * time.Minute

func (s *uniboxService) publishSeenRelay(ctx context.Context, orgID uuid.UUID, changed []uuid.UUID) {
	targets, err := s.uniboxRepository.SeenRelayTargets(ctx, orgID, changed)
	if err != nil {
		errs.CaptureException(err)
		return
	}

	// One event per mailbox and read state: "mark all as read" on a busy
	// folder is one press over thousands of messages, a mailbox is the unit a
	// worker holds, and a batch carries one state for all of it.
	type relayKey struct {
		emailID uuid.UUID
		seen    bool
	}
	batches := make(map[relayKey]*models.MessageSeenAction)
	workers := make(map[uuid.UUID]uuid.UUID, len(targets))
	order := make([]relayKey, 0, len(targets))
	for _, t := range targets {
		key := relayKey{emailID: t.EmailID, seen: t.Seen}
		act, ok := batches[key]
		if !ok {
			act = &models.MessageSeenAction{EmailID: t.EmailID, Seen: t.Seen}
			batches[key] = act
			workers[t.EmailID] = t.WorkerID
			order = append(order, key)
		}
		act.Messages = append(act.Messages, t.Ref)
	}

	for _, key := range order {
		act := batches[key]
		for start := 0; start < len(act.Messages); start += models.SeenRelayChunk {
			end := start + models.SeenRelayChunk
			if end > len(act.Messages) {
				end = len(act.Messages)
			}
			batch := &models.MessageSeenAction{
				EmailID:  key.emailID,
				Seen:     key.seen,
				Messages: act.Messages[start:end],
			}
			if err := s.publisher.PublishMessageSeen(ctx, workers[key.emailID], batch); err != nil {
				log.Warn().Err(err).
					Str("email_account_id", key.emailID.String()).
					Int("messages", len(batch.Messages)).
					Msg("could not relay the unibox read state to the mailbox provider")
			}
		}
	}
}

// MoveFolderBulk backs Archive, Delete and Move to inbox. The store is filed
// first; provider_folder waits for the worker's answer to the relay.
func (s *uniboxService) MoveFolderBulk(ctx context.Context, orgID uuid.UUID, data *models.MoveFolder) (*models.MoveFolder, *errx.Error) {
	if len(data.EmailIDs) > 500 || len(data.ThreadIDs) > 500 {
		return nil, errx.ErrSeenMax
	}
	// Only the three a user can file into. sent/drafts/spam are verdicts the
	// provider reaches, and accepting them here would let a caller forge one.
	if !models.FilableFolder(data.Folder) {
		return nil, errx.ErrUniboxFilableFolder
	}
	// Filing by conversation is what the list rows use; the reader still names
	// the messages it has loaded.
	byThread, err := s.uniboxRepository.MoveThreadsToFolder(ctx, orgID, data.ThreadIDs, data.Folder)
	if err != nil {
		errs.CaptureException(err)
		return nil, errx.InternalError()
	}
	byID, err := s.uniboxRepository.MoveToFolderBulk(ctx, orgID, data.EmailIDs, data.Folder)
	if err != nil {
		errs.CaptureException(err)
		return nil, errx.InternalError()
	}
	s.relayFolder(ctx, orgID, append(byThread, byID...))
	return data, nil
}

// relayFolder carries a filing out to the mailboxes, detached and never
// retried, like relaySeen.
func (s *uniboxService) relayFolder(ctx context.Context, orgID uuid.UUID, filed []models.FiledMessage) {
	if s.publisher == nil || len(filed) == 0 {
		return
	}
	go func() {
		bg, cancel := context.WithTimeout(context.WithoutCancel(ctx), seenRelayTimeout)
		defer cancel()
		s.publishFolderRelay(bg, orgID, filed)
	}()
}

func (s *uniboxService) publishFolderRelay(ctx context.Context, orgID uuid.UUID, filed []models.FiledMessage) {
	// A row named by thread and by id comes back twice, unmoved the second
	// time, so either report moving it counts.
	ids := make([]uuid.UUID, 0, len(filed))
	moved := make(map[uuid.UUID]bool, len(filed))
	for _, f := range filed {
		if _, ok := moved[f.ID]; !ok {
			ids = append(ids, f.ID)
		}
		moved[f.ID] = moved[f.ID] || f.Moved
	}
	targets, err := s.uniboxRepository.FolderRelayTargets(ctx, orgID, ids)
	if err != nil {
		errs.CaptureException(err)
		return
	}

	// The destination is the folder each row holds now, so concurrent filings
	// leave the provider agreeing with the store.
	type relayKey struct {
		emailID uuid.UUID
		folder  string
	}
	batches := make(map[relayKey][]models.MessageFolderRef)
	workers := make(map[uuid.UUID]uuid.UUID, len(targets))
	var order []relayKey
	for _, t := range targets {
		if !models.RelaysFolderMove(t.Provider, t.Ref.ProviderFolder, t.Folder, moved[t.Ref.ID]) {
			continue
		}
		key := relayKey{emailID: t.EmailID, folder: t.Folder}
		if _, ok := batches[key]; !ok {
			workers[t.EmailID] = t.WorkerID
			order = append(order, key)
		}
		batches[key] = append(batches[key], t.Ref)
	}

	for _, key := range order {
		refs := batches[key]
		for start := 0; start < len(refs); start += models.FolderRelayChunk {
			end := min(start+models.FolderRelayChunk, len(refs))
			batch := &models.MessageFolderAction{EmailID: key.emailID, Folder: key.folder, Messages: refs[start:end]}
			if err := s.publisher.PublishMessageFolder(ctx, workers[key.emailID], batch); err != nil {
				log.Warn().Err(err).
					Str("email_account_id", key.emailID.String()).
					Str("folder", key.folder).
					Int("messages", len(batch.Messages)).
					Msg("could not relay the unibox filing to the mailbox provider")
			}
		}
	}
}
