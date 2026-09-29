package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/jobrun"
	"github.com/warmbly/warmbly/internal/models"
)

// checkWarmupRemoval asks the worker holding the mailbox where a removed
// warmup message went; a removal is only a message leaving a folder.
func (s *JobsService) checkWarmupRemoval(ctx context.Context, userID, accountID uuid.UUID, rfcMessageID string) error {
	if s.Publisher == nil || s.EmailRepository == nil || rfcMessageID == "" {
		return nil
	}
	account, xerr := s.EmailRepository.GetByID(ctx, accountID)
	if xerr != nil {
		if xerr.Code == errx.NotFound {
			return nil
		}
		return fmt.Errorf("warmup removal check: mailbox lookup: %w", xerr)
	}
	if account == nil || account.WorkerID == nil {
		log.Info().Str("email_id", accountID.String()).Msg("Warmup removal not checked: mailbox has no worker; nothing charged")
		return nil
	}
	return s.publishRemovalCheck(ctx, *account.WorkerID, userID, accountID, rfcMessageID)
}

func (s *JobsService) publishRemovalCheck(ctx context.Context, workerID, userID, accountID uuid.UUID, rfcMessageID string) error {
	if err := s.Publisher.PublishWarmupAction(ctx, workerID, &models.WarmupEmailAction{
		UserID:       userID,
		EmailID:      accountID,
		RFCMessageID: rfcMessageID,
		Actions:      []string{models.WarmupActionVerifyRemoval},
	}); err != nil {
		return fmt.Errorf("warmup removal check: publish: %w", err)
	}
	return nil
}

// HandleWarmupRemovalChecked judges a removal on where the worker found the
// message: outside the trash withdraws any strike for it, in the trash or
// nowhere records one.
func (s *JobsService) HandleWarmupRemovalChecked(ctx context.Context, e *models.JobEventWarmupRemovalChecked) error {
	if s.WarmupService == nil || e == nil || e.RFCMessageID == "" {
		return nil
	}
	var health *models.WarmupParticipantHealth
	var xerr *errx.Error
	switch e.Outcome {
	case models.WarmupRemovalPresent:
		health, xerr = s.WarmupService.WithdrawTampering(ctx, e.EmailID, e.RFCMessageID, "deletion")
	case models.WarmupRemovalTrashed, models.WarmupRemovalGone:
		health, xerr = s.WarmupService.RecordTampering(ctx, e.EmailID, e.RFCMessageID, "deletion")
		if xerr == nil && s.WarmupRepo != nil {
			// Confirms a strike recorded before the search existed.
			if err := s.WarmupRepo.MarkTamperingVerified(ctx, e.EmailID, e.RFCMessageID, "deletion"); err != nil {
				return fmt.Errorf("confirm warmup strike: %w", err)
			}
		}
	default:
		return nil
	}
	if xerr != nil {
		return fmt.Errorf("judge warmup removal: %w", xerr)
	}
	s.markRiskBandFromWarmupHealth(ctx, e.EmailID, health)
	return nil
}

const (
	warmupTamperingRecheckBatch = 100
	// The seven days a strike counts plus the thirty-day block it can impose.
	warmupTamperingRecheckWindow = 37 * 24 * time.Hour
	// A search nobody answered is asked for again after this.
	warmupTamperingRecheckRetry = 6 * time.Hour
)

// StartWarmupTamperingRecheck searches for every deletion strike recorded
// before removals were checked, until each is answered or ages out.
func (s *JobsService) StartWarmupTamperingRecheck(ctx context.Context) {
	if s.WarmupRepo == nil || s.Publisher == nil {
		return
	}
	jobrun.Loop(ctx, "warmup_tampering_recheck", time.Hour, true, func(ctx context.Context) error {
		batchCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		defer cancel()
		return s.recheckTamperingBatch(batchCtx)
	})
}

func (s *JobsService) recheckTamperingBatch(ctx context.Context) error {
	rows, err := s.WarmupRepo.ListUnverifiedDeletions(ctx, time.Now().Add(-warmupTamperingRecheckWindow), warmupTamperingRecheckRetry, warmupTamperingRecheckBatch)
	if err != nil {
		return err
	}
	var failures []error
	for i := range rows {
		r := &rows[i]
		if err := s.publishRemovalCheck(ctx, r.WorkerID, r.UserID, r.EmailAccountID, r.MessageID); err != nil {
			failures = append(failures, err)
			continue
		}
		if err := s.WarmupRepo.MarkTamperingVerifyRequested(ctx, r.EmailAccountID, r.MessageID); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
