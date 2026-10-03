package jobs

import (
	"context"

	"github.com/rs/zerolog/log"
	"github.com/warmbly/warmbly/internal/models"
)

// HandleInboundComplaint records an abuse feedback report (parsed worker-side)
// against the original campaign send: suppress the recipient, mark the send
// complained, and feed the deliverability breaker. A report that genuinely
// cannot be attributed (unknown/pruned/non-Warmbly send, warmup, cross-mailbox,
// forged recipient) reads back nil and is acked; a technical failure reads back
// an error and is returned so the bus redelivers it (bounded by the consumer's
// MaxDeliver) rather than dropping an otherwise attributable report.
func (s *JobsService) HandleInboundComplaint(ctx context.Context, e *models.JobEventInboundComplaint) error {
	if s.AdvancedService == nil {
		return nil
	}
	if xerr := s.AdvancedService.RecordInboundComplaint(ctx, e.EmailID, e.OriginalMessageID, e.ComplainedRecipient, e.Provider); xerr != nil {
		log.Warn().
			Str("email_id", e.EmailID.String()).
			Str("original_message_id", e.OriginalMessageID).
			Str("error", xerr.Message).
			Msg("Failed to record inbound complaint; leaving it for redelivery")
		return xerr
	}
	return nil
}
