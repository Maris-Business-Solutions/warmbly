package advanced

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/pkg/mailhdr"
)

// RecordInboundBounce turns a permanent NDR the worker parsed into a bounce
// deliverability event. The worker only emits permanent (5.x.x) bounces with an
// original Message-ID, so this resolves that id to the campaign send and routes
// it through IngestDeliverabilityEvent (suppression + campaign progress +
// breaker), keyed idempotently so re-delivered NDRs don't double-count.
func (s *service) RecordInboundBounce(ctx context.Context, emailAccountID uuid.UUID, originalMessageID, failedRecipient, reason string) *errx.Error {
	originalMessageID = strings.Trim(strings.TrimSpace(originalMessageID), "<>")
	if originalMessageID == "" {
		return nil
	}

	task, err := s.taskRepo.GetTaskByMessageID(ctx, originalMessageID)
	if err != nil || task == nil {
		// Unknown message id (a non-Warmbly send, or already pruned) — nothing
		// to attribute the bounce to.
		return nil
	}

	// A warmup send's NDR is warmup's business and nobody else's. Warmup tasks
	// carry a message_id exactly like campaign tasks, so this used to resolve
	// and then suppress a POOL PARTNER'S address in the customer's suppression
	// list and record the bounce against their deliverability, where it fed the
	// breaker. Warmup bounce rate is already a band in the warmup health model.
	if task.TaskType == models.TaskTypeWarmup {
		return nil
	}

	// The NDR should have landed in the same mailbox that sent it; if it didn't
	// resolve to this account, don't attribute a cross-account bounce.
	if task.EmailAccountID != emailAccountID {
		return nil
	}

	account, aerr := s.emailRepo.GetByID(ctx, emailAccountID)
	if aerr != nil || account == nil || account.OrganizationID == nil {
		return nil
	}

	req := &models.IngestDeliverabilityEventRequest{
		EventType:      models.DeliverabilityEventBounce,
		Provider:       "inbound_ndr",
		TaskID:         &task.ID,
		RecipientEmail: failedRecipient,
		Reason:         reason,
		// Same NDR re-synced (delta re-runs, reconnects) must not double-count.
		IdempotencyKey: "ndr:" + originalMessageID,
	}

	if ct, cerr := s.taskRepo.GetCampaignTask(ctx, task.ID); cerr == nil && ct != nil {
		req.CampaignID = ct.CampaignID
		req.ContactID = ct.ContactID
		if ct.ContactID != nil {
			if contact, cerr := s.contactRepo.GetByID(ctx, *ct.ContactID); cerr == nil && contact != nil {
				switch {
				case req.RecipientEmail == "":
					req.RecipientEmail = contact.Email
				case ct.CampaignID != nil && !strings.EqualFold(mailhdr.Bare(req.RecipientEmail), strings.TrimSpace(contact.Email)):
					if owner, isCopy := s.copyBounceOwner(ctx, *ct.CampaignID, *ct.ContactID, req.RecipientEmail); isCopy {
						// A copy bounced, not the lead: the lead keeps its
						// sequence, and the copy's own NDR is its own event.
						req.ContactID = owner
						req.IdempotencyKey += ":" + strings.ToLower(mailhdr.Bare(req.RecipientEmail))
					}
				}
			}
		}
	}

	if req.RecipientEmail == "" {
		// IngestDeliverabilityEvent requires a recipient; without one we can't
		// suppress or record. Give up rather than guess.
		return nil
	}

	return s.IngestDeliverabilityEvent(ctx, *account.OrganizationID, req)
}

// RecordInboundComplaint turns an abuse feedback report the worker parsed into
// a complaint deliverability event, resolved against the original send. Keyed
// idempotently so a re-synced report cannot double-count.
func (s *service) RecordInboundComplaint(ctx context.Context, emailAccountID uuid.UUID, originalMessageID, complainedRecipient, provider string) *errx.Error {
	originalMessageID = strings.Trim(strings.TrimSpace(originalMessageID), "<>")
	if originalMessageID == "" {
		return nil
	}

	task, err := s.taskRepo.GetTaskByMessageID(ctx, originalMessageID)
	if err != nil || task == nil {
		// A non-Warmbly send, or already pruned.
		return nil
	}
	// Warmup never reaches the customer's deliverability record. The campaign
	// task below would refuse it anyway; refusing it here says so once.
	if task.TaskType == models.TaskTypeWarmup {
		return nil
	}
	if task.EmailAccountID != emailAccountID {
		// The report should reach the mailbox that sent it; do not attribute a
		// complaint across accounts.
		return nil
	}

	account, aerr := s.emailRepo.GetByID(ctx, emailAccountID)
	if aerr != nil || account == nil || account.OrganizationID == nil {
		return nil
	}

	if provider == "" {
		provider = "inbound_fbl"
	}
	req := &models.IngestDeliverabilityEventRequest{
		EventType:      models.DeliverabilityEventComplaint,
		Provider:       provider,
		TaskID:         &task.ID,
		Reason:         "recipient reported the message as spam",
		IdempotencyKey: "fbl:" + originalMessageID,
	}

	// Who gets suppressed comes from the RESOLVED SEND, never from the report.
	// A report is unauthenticated mail that anyone able to reach this mailbox
	// could forge; honouring the address it names would let a forger suppress
	// a contact that send never went to. Bounded this way, the worst a forged
	// report can do is suppress the one contact who actually received it,
	// which is the same person a genuine complaint would suppress.
	ct, cerr := s.taskRepo.GetCampaignTask(ctx, task.ID)
	if cerr != nil || ct == nil || ct.ContactID == nil {
		return nil
	}
	req.CampaignID = ct.CampaignID
	req.ContactID = ct.ContactID

	contact, conErr := s.contactRepo.GetByID(ctx, *ct.ContactID)
	if conErr != nil || contact == nil || contact.Email == "" {
		return nil
	}
	req.RecipientEmail = contact.Email

	// When the provider did disclose an address, it must be the one we sent to.
	// A mismatch means the report is not about this send.
	if complainedRecipient != "" && !strings.EqualFold(strings.TrimSpace(complainedRecipient), contact.Email) {
		return nil
	}

	return s.IngestDeliverabilityEvent(ctx, *account.OrganizationID, req)
}

// copyBounceOwner tells a bounced copy apart from the lead. A contact copied on
// the lead is marked bounced there and owns the event; an address from the
// campaign's own CC or BCC owns it with no contact. Any other address (a
// forward, an alias) stays the lead's, as before.
func (s *service) copyBounceOwner(ctx context.Context, campaignID, leadID uuid.UUID, address string) (*uuid.UUID, bool) {
	bare := mailhdr.Bare(address)
	if s.campaignProgressRepo != nil {
		if id, err := s.campaignProgressRepo.MarkLeadCCBounced(ctx, campaignID, leadID, bare); err == nil && id != nil {
			return id, true
		}
	}
	if campaign, err := s.campaignRepo.GetByID(ctx, campaignID); err == nil && campaign != nil {
		for _, list := range [][]string{campaign.CC, campaign.BCC} {
			for _, a := range list {
				if strings.EqualFold(mailhdr.Bare(a), bare) {
					return nil, true
				}
			}
		}
	}
	return &leadID, false
}
