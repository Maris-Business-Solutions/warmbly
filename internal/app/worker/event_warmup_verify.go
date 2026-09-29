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

// verifyWarmupRemoval reports where a warmup message the sync saw removed is.
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
	return w.Produce(models.JobEventTypeWarmupRemovalChecked, action.EmailID.String(), &models.JobEventWarmupRemovalChecked{
		UserID:       action.UserID,
		EmailID:      action.EmailID,
		RFCMessageID: action.RFCMessageID,
		Outcome:      outcome,
		Recheck:      action.Recheck,
	})
}

// locateWarmupMessage searches the whole mailbox by Message-ID; trashed means
// every copy found is in the trash.
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

// imapMessageLocator is the slice of the IMAP client the search needs.
type imapMessageLocator interface {
	LocateMessageID(ctx context.Context, mailboxes []string, rfcMessageID string) (held []string, unsearched int, err error)
}

// locateImapMessage searches every synced folder. Not found, or found only in
// the trash with a folder left unsearched, cannot be told apart from a move
// into a folder the sync does not list.
func locateImapMessage(ctx context.Context, client imapMessageLocator, boxes []*models.Mailbox, rfcMessageID string) (string, error) {
	var names []string
	trash := map[string]bool{}
	for _, b := range boxes {
		if b == nil || !imap.SelectableFolder(b.Attrs) {
			continue
		}
		names = append(names, b.Name)
		trash[b.Name] = imap.IsTrashMailbox(b.Name, b.Attrs)
	}
	held, unsearched, err := client.LocateMessageID(ctx, names, rfcMessageID)
	if err != nil {
		return "", err
	}
	for _, name := range held {
		if !trash[name] {
			return models.WarmupRemovalPresent, nil
		}
	}
	if len(held) > 0 && unsearched == 0 {
		return models.WarmupRemovalTrashed, nil
	}
	return models.WarmupRemovalUnknown, nil
}
