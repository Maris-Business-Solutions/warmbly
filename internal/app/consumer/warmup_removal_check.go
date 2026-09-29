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

// A removal the sync reports is only a message leaving a folder. Graph reports
// every move that way, and a message filed elsewhere by a provider's filter, a
// mailbox rule, another Warmbly instance syncing the same mailbox or our own
// filing is still in the mailbox. So a fresh removal of warmup mail is never a
// strike on its own: the worker searches the mailbox for the message and a
// strike is recorded only when it is in the trash or gone.

// checkWarmupRemoval asks the worker holding the mailbox where a removed
// warmup message went. A mailbox with nowhere to ask is not charged.
func (s *JobsService) checkWarmupRemoval(ctx context.Context, userID, accountID uuid.UUID, rfcMessageID string, internalID *uuid.UUID) error {
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
	return s.publishRemovalCheck(ctx, *account.WorkerID, userID, accountID, rfcMessageID, internalID)
}

func (s *JobsService) publishRemovalCheck(ctx context.Context, workerID, userID, accountID uuid.UUID, rfcMessageID string, internalID *uuid.UUID) error {
	action := &models.WarmupEmailAction{
		UserID:       userID,
		EmailID:      accountID,
		RFCMessageID: rfcMessageID,
		Actions:      []string{models.WarmupActionVerifyRemoval},
	}
	if internalID != nil && *internalID != uuid.Nil {
		action.InternalID = internalID.String()
	}
	if err := s.Publisher.PublishWarmupAction(ctx, workerID, action); err != nil {
		return fmt.Errorf("warmup removal check: publish: %w", err)
	}
	return nil
}

// HandleWarmupRemovalChecked judges a removal on where the worker found the
// message. Found anywhere outside the trash withdraws any strike for it,
// which is also how a strike recorded before this check is corrected.
func (s *JobsService) HandleWarmupRemovalChecked(ctx context.Context, e *models.JobEventWarmupRemovalChecked) error {
	if s.WarmupService == nil || e == nil || e.RFCMessageID == "" {
		return nil
	}
	withdraw := e.Outcome == models.WarmupRemovalPresent
	switch e.Outcome {
	case models.WarmupRemovalPresent:
	case models.WarmupRemovalTrashed, models.WarmupRemovalGone:
		// The retention sweep deletes warmup mail itself; a message it has
		// retired being gone says nothing about the owner.
		withdraw = s.retiredByPlatform(ctx, e)
	default:
		return nil
	}

	if withdraw {
		health, xerr := s.WarmupService.WithdrawTampering(ctx, e.EmailID, e.RFCMessageID, "deletion")
		if xerr != nil {
			return fmt.Errorf("withdraw warmup strike: %w", xerr)
		}
		s.markRiskBandFromWarmupHealth(ctx, e.EmailID, health)
		return nil
	}
	health, _ := s.WarmupService.RecordTampering(ctx, e.EmailID, e.RFCMessageID, "deletion")
	s.markRiskBandFromWarmupHealth(ctx, e.EmailID, health)
	return nil
}

func (s *JobsService) retiredByPlatform(ctx context.Context, e *models.JobEventWarmupRemovalChecked) bool {
	internalID, err := uuid.Parse(e.InternalID)
	if err != nil || s.WarmupRepo == nil {
		return false
	}
	rec, _ := s.WarmupRepo.GetWarmupReceived(ctx, e.EmailID, internalID)
	return rec != nil && rec.RetiredAt != nil
}

const (
	warmupTamperingRecheckBatch = 100
	// warmupTamperingRecheckWindow is the seven days a strike counts plus the
	// thirty-day block it can lead to; an older strike decides nothing.
	warmupTamperingRecheckWindow = 37 * 24 * time.Hour
)

// StartWarmupTamperingRecheck searches the mailbox once for every deletion
// strike recorded before removals were checked, so a strike for a message
// that was only moved is withdrawn and the hold it caused is lifted.
func (s *JobsService) StartWarmupTamperingRecheck(ctx context.Context) {
	if s.WarmupRepo == nil || s.Publisher == nil {
		return
	}
	jobrun.Loop(ctx, "warmup_tampering_recheck", 10*time.Minute, true, func(ctx context.Context) error {
		batchCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		defer cancel()
		return s.recheckTamperingBatch(batchCtx)
	})
}

// recheckTamperingBatch stamps a strike once its search is on the bus; a
// search that never answers leaves the strike as it was.
func (s *JobsService) recheckTamperingBatch(ctx context.Context) error {
	rows, err := s.WarmupRepo.ListUnverifiedDeletions(ctx, time.Now().Add(-warmupTamperingRecheckWindow), warmupTamperingRecheckBatch)
	if err != nil {
		return err
	}
	var failures []error
	for i := range rows {
		r := &rows[i]
		if err := s.publishRemovalCheck(ctx, r.WorkerID, r.UserID, r.EmailAccountID, r.MessageID, r.InternalID); err != nil {
			failures = append(failures, err)
			continue
		}
		if err := s.WarmupRepo.MarkTamperingVerified(ctx, r.EmailAccountID, r.MessageID, "deletion"); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
