package wmail

import (
	"context"
	"slices"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/client/goog"
	"github.com/warmbly/warmbly/internal/models"
)

func (w *WMail) onGoogleMessageRemove(ctx context.Context, messageID string) error {
	internalMessage, err := w.EmailMessageMapRepository.Get(ctx, w.UserID, w.ID, messageID)
	if err != nil {
		return err
	}

	if internalMessage == nil {
		return nil
	}

	internalID, err := uuid.Parse(internalMessage.ID)
	if err != nil {
		return err
	}

	if err := w.onEvent(models.JobEventTypeRemoveEmail, &models.JobEventRemoveEmail{
		UserID:  w.UserID,
		EmailID: w.ID,
		ID:      internalID,
	}); err != nil {
		return err
	}

	return nil
}

// translateGmailLabels maps a Gmail label transition onto internal flag
// add/remove sets. Gmail models read state inversely (the UNREAD label marks
// unread mail), so gaining UNREAD removes \Seen and losing it adds \Seen.
// Unmapped labels pass through in the transition's own direction.
func translateGmailLabels(labelIDs []string, added bool) (addFlags, removeFlags []string) {
	for _, label := range labelIDs {
		var flag string
		inverted := false
		switch label {
		case "UNREAD":
			flag, inverted = "\\Seen", true
		case "STARRED":
			flag = "\\Flagged"
		case "IMPORTANT":
			flag = "\\Important"
		case "DRAFT":
			flag = "\\Draft"
		default:
			flag = label
		}
		if added != inverted {
			addFlags = append(addFlags, flag)
		} else {
			removeFlags = append(removeFlags, flag)
		}
	}
	return addFlags, removeFlags
}

func (w *WMail) onGoogleMessageLabelsAdded(ctx context.Context, messageID string, changed, current []string) error {
	return w.emitGoogleLabelEvents(ctx, messageID, changed, current, true)
}

func (w *WMail) onGoogleMessageLabelsRemoved(ctx context.Context, messageID string, changed, current []string) error {
	return w.emitGoogleLabelEvents(ctx, messageID, changed, current, false)
}

func (w *WMail) emitGoogleLabelEvents(ctx context.Context, messageID string, changed, current []string, added bool) error {
	internalMessage, err := w.EmailMessageMapRepository.Get(ctx, w.UserID, w.ID, messageID)
	if err != nil {
		return err
	}

	if internalMessage == nil {
		return nil
	}

	internalID, err := uuid.Parse(internalMessage.ID)
	if err != nil {
		return err
	}

	addFlags, removeFlags := translateGmailLabels(changed, added)

	if len(addFlags) > 0 {
		if err := w.onEvent(models.JobEventTypeFlagsAdd, &models.JobEventFlags{
			UserID:  w.UserID,
			EmailID: w.ID,
			ID:      internalID,
			Flags:   addFlags,
		}); err != nil {
			return err
		}
	}
	if len(removeFlags) > 0 {
		if err := w.onEvent(models.JobEventTypeFlagsRemove, &models.JobEventFlags{
			UserID:  w.UserID,
			EmailID: w.ID,
			ID:      internalID,
			Flags:   removeFlags,
		}); err != nil {
			return err
		}
	}

	if !slices.ContainsFunc(changed, goog.IsFolderLabel) {
		return nil
	}
	// Archive, Delete, Report spam and Move to inbox are label changes in
	// Gmail, so the folder is recomputed from the labels the message now has.
	// Labels that contradict the change are not trusted and are looked up.
	labels := current
	if labels == nil || !labelsReflect(changed, labels, added) {
		found := false
		labels, found, err = w.GoogleData.Client.MessageLabels(ctx, messageID)
		if err != nil {
			return err
		}
		if !found {
			return nil
		}
	}
	return w.onEvent(models.JobEventTypeFolderUpdate, &models.JobEventFolderUpdate{
		UserID:  w.UserID,
		EmailID: w.ID,
		ID:      internalID,
		Folder:  goog.Folder(labels),
	})
}

// labelsReflect reports whether labels already show the folder labels in
// changed as added (or removed).
func labelsReflect(changed, labels []string, added bool) bool {
	for _, l := range changed {
		if goog.IsFolderLabel(l) && slices.Contains(labels, l) != added {
			return false
		}
	}
	return true
}
