package jobs

import (
	"context"

	"github.com/rs/zerolog/log"
	"github.com/warmbly/warmbly/internal/models"
)

// HandleInboundBounce records a permanent NDR (parsed worker-side) against the
// original campaign send: suppress the recipient, mark the send bounced, and
// feed the deliverability breaker. A bounce that genuinely cannot be attributed
// (unknown/pruned/non-Warmbly send, warmup, cross-mailbox) reads back nil and is
// acked; a technical failure reads back an error and is returned so the bus
// redelivers it (bounded by the consumer's MaxDeliver) rather than dropping an
// otherwise attributable notice.
func (s *JobsService) HandleInboundBounce(ctx context.Context, e *models.JobEventInboundBounce) error {
	if s.AdvancedService == nil {
		return nil
	}
	if xerr := s.AdvancedService.RecordInboundBounce(ctx, e.EmailID, e.OriginalMessageID, e.FailedRecipient, e.Reason); xerr != nil {
		log.Warn().
			Str("email_id", e.EmailID.String()).
			Str("original_message_id", e.OriginalMessageID).
			Str("error", xerr.Message).
			Msg("Failed to record inbound bounce; leaving it for redelivery")
		return xerr
	}
	return nil
}
