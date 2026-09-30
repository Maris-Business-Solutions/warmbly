package goog

import (
	"context"
	"errors"
	"fmt"

	"github.com/warmbly/warmbly/internal/models"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/googleapi"
)

// FolderLabels is the label change that puts a message in a canonical
// folder, the way Gmail's own buttons do: Archive drops INBOX, Delete adds
// TRASH, Move to inbox adds INBOX back. Leaving the trash or spam is part of
// every other destination.
func FolderLabels(folder string) (add, remove []string, ok bool) {
	switch folder {
	case models.FolderInbox:
		return []string{Inbox}, []string{Trash, Spam}, true
	case models.FolderArchive:
		return nil, []string{Inbox, Trash, Spam}, true
	case models.FolderTrash:
		return []string{Trash}, []string{Spam}, true
	}
	return nil, nil, false
}

// MoveToFolder files many messages into a canonical folder with one
// batchModify. Gmail accepts up to 1000 ids per request; the caller chunks.
// A refusal of the TRASH label falls back to messages.trash and untrash,
// which are the documented way in and out of the trash.
func (c *Client) MoveToFolder(ctx context.Context, messageIDs []string, folder string) error {
	if c.srv == nil {
		return fmt.Errorf("gmail service not initialized")
	}
	add, remove, ok := FolderLabels(folder)
	if !ok {
		return fmt.Errorf("gmail: %q is not a folder a message can be filed into", folder)
	}
	if len(messageIDs) == 0 {
		return nil
	}
	req := &gmail.BatchModifyMessagesRequest{Ids: messageIDs, AddLabelIds: add, RemoveLabelIds: remove}
	err := c.srv.Users.Messages.BatchModify("me", req).Context(ctx).Do()
	if err == nil {
		return nil
	}
	if !isLabelRefusal(err) {
		return fmt.Errorf("failed to move %d messages to %s: %w", len(messageIDs), folder, err)
	}
	for _, id := range messageIDs {
		if err := c.moveOne(ctx, id, folder, add, remove); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) moveOne(ctx context.Context, id, folder string, add, remove []string) error {
	if folder == models.FolderTrash {
		return c.Trash(ctx, id)
	}
	if _, err := c.srv.Users.Messages.Untrash("me", id).Context(ctx).Do(); err != nil && !isNotFound(err) {
		return fmt.Errorf("failed to take a message out of the trash: %w", err)
	}
	if err := c.modifyLabels(ctx, id, add, withoutLabel(remove, Trash)); err != nil && !isNotFound(err) {
		return fmt.Errorf("failed to move a message to %s: %w", folder, err)
	}
	return nil
}

func withoutLabel(labels []string, drop string) []string {
	out := make([]string, 0, len(labels))
	for _, l := range labels {
		if l != drop {
			out = append(out, l)
		}
	}
	return out
}

// isNotFound is Gmail answering for a message it no longer has.
func isNotFound(err error) bool {
	var gerr *googleapi.Error
	return errors.As(err, &gerr) && gerr.Code == 404
}
