package worker

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/warmbly/warmbly/internal/app/worker/wmail"
	"github.com/warmbly/warmbly/internal/client/smtpimap/imap"
	"github.com/warmbly/warmbly/internal/models"
)

// verifyWarmupRemoval looks for a warmup message the sync reported removed and
// reports where it is. A failed lookup is an error so the bus offers it again;
// one that never succeeds, or cannot tell, reports nothing and nothing is
// charged.
func (w *WorkerService) verifyWarmupRemoval(ctx context.Context, mail *wmail.WMail, action models.WarmupEmailAction) error {
	outcome, err := locateWarmupMessage(ctx, mail, action.RFCMessageID)
	if err != nil {
		return fmt.Errorf("verify warmup removal: %w", err)
	}
	log.Debug().
		Str("email_id", action.EmailID.String()).
		Str("rfc_message_id", action.RFCMessageID).
		Str("outcome", outcome).
		Msg("Checked where a removed warmup message went")
	if outcome == "" {
		return nil
	}
	return w.Produce(models.JobEventTypeWarmupRemovalChecked, action.EmailID.String(), &models.JobEventWarmupRemovalChecked{
		UserID:       action.UserID,
		EmailID:      action.EmailID,
		InternalID:   action.InternalID,
		RFCMessageID: action.RFCMessageID,
		Outcome:      outcome,
	})
}

// locateWarmupMessage searches the whole mailbox by Message-ID. Trashed means
// every copy found is in the trash; "" means the search cannot tell.
func locateWarmupMessage(ctx context.Context, mail *wmail.WMail, rfcMessageID string) (string, error) {
	switch {
	case mail.GoogleData != nil && mail.GoogleData.Client != nil:
		return removalOutcome(mail.GoogleData.Client.LocateRFCMessageID(ctx, rfcMessageID))
	case mail.GraphData != nil && mail.GraphData.Client != nil:
		return removalOutcome(mail.GraphData.Client.LocateRFCMessageID(ctx, rfcMessageID))
	case mail.SmtpImapData != nil && mail.SmtpImapData.ImapClient != nil:
		return locateImapMessage(ctx, mail.SmtpImapData.ImapClient, mail.SmtpImapData.Mailboxes, rfcMessageID)
	}
	return "", errors.New("no mail client to search")
}

func removalOutcome(found, trashed bool, err error) (string, error) {
	switch {
	case err != nil:
		return "", err
	case !found:
		return models.WarmupRemovalGone, nil
	case trashed:
		return models.WarmupRemovalTrashed, nil
	}
	return models.WarmupRemovalPresent, nil
}

// imapMessageHolder is the slice of the IMAP client the search needs.
type imapMessageHolder interface {
	HoldsMessageID(ctx context.Context, mailboxName, rfcMessageID string) (bool, error)
}

// locateImapMessage asks every synced folder, trash last. Not finding it is
// inconclusive: the synced list leaves out folders the owner excluded and
// folders past the sync's cap.
func locateImapMessage(ctx context.Context, client imapMessageHolder, boxes []*models.Mailbox, rfcMessageID string) (string, error) {
	var trash []string
	for _, b := range boxes {
		if b == nil || !imap.SelectableFolder(b.Attrs) {
			continue
		}
		if imap.IsTrashMailbox(b.Name, b.Attrs) {
			trash = append(trash, b.Name)
			continue
		}
		held, err := client.HoldsMessageID(ctx, b.Name, rfcMessageID)
		if err != nil {
			return "", err
		}
		if held {
			return models.WarmupRemovalPresent, nil
		}
	}
	for _, name := range trash {
		held, err := client.HoldsMessageID(ctx, name, rfcMessageID)
		if err != nil {
			return "", err
		}
		if held {
			return models.WarmupRemovalTrashed, nil
		}
	}
	return "", nil
}
