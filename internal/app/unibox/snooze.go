package unibox

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/observability/errs"
)

// Snooze takes a set of conversations so the list's selection bar is one call.
// A single thread is the one-element case.
func (s *uniboxService) Snooze(ctx context.Context, orgID, userID uuid.UUID, threadIDs []string, until time.Time) ([]models.UniboxSnooze, *errx.Error) {
	threadIDs = nonEmpty(threadIDs)
	if len(threadIDs) == 0 {
		return nil, errx.New(errx.BadRequest, "thread_id is required")
	}
	if len(threadIDs) > SnoozeMaxThreads {
		return nil, errx.ErrSeenMax
	}
	now := time.Now()
	// Tiny lead-time grace so a click that takes a few hundred ms
	// over the wire doesn't blow up validation when the user picked
	// "in 1 minute" exactly.
	if until.Before(now.Add(5 * time.Second)) {
		return nil, errx.New(errx.BadRequest, "snoozed_until must be in the future")
	}
	if until.After(now.Add(SnoozeMaxHorizon)) {
		return nil, errx.New(errx.BadRequest, "snoozed_until is too far in the future (max 90 days)")
	}

	rows, err := s.uniboxRepository.UpsertSnoozes(ctx, orgID, userID, threadIDs, until.UTC())
	if err != nil {
		errs.CaptureException(err)
		return nil, errx.InternalError()
	}
	return rows, nil
}

func (s *uniboxService) Unsnooze(ctx context.Context, orgID, userID uuid.UUID, threadIDs []string) *errx.Error {
	threadIDs = nonEmpty(threadIDs)
	if len(threadIDs) == 0 {
		return errx.New(errx.BadRequest, "thread_id is required")
	}
	if len(threadIDs) > SnoozeMaxThreads {
		return errx.ErrSeenMax
	}
	if err := s.uniboxRepository.DeleteSnoozes(ctx, orgID, userID, threadIDs); err != nil {
		errs.CaptureException(err)
		return errx.InternalError()
	}
	return nil
}

// nonEmpty drops blank ids, so one empty string in a list is not a request to
// snooze a conversation that does not exist.
func nonEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

func (s *uniboxService) ListSnoozes(ctx context.Context, orgID, userID uuid.UUID) ([]models.UniboxSnooze, *errx.Error) {
	rows, err := s.uniboxRepository.ListSnoozes(ctx, orgID, userID)
	if err != nil {
		errs.CaptureException(err)
		return nil, errx.InternalError()
	}
	return rows, nil
}
