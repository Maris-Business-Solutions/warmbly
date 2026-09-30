package msgraph

import (
	"context"
	"fmt"

	"github.com/warmbly/warmbly/internal/models"
)

// WellKnownFolder is the Outlook folder a canonical unibox folder files into.
func WellKnownFolder(folder string) (string, bool) {
	switch folder {
	case models.FolderInbox:
		return FolderInbox, true
	case models.FolderArchive:
		return FolderArchive, true
	case models.FolderTrash:
		return FolderDeletedItems, true
	}
	return "", false
}

// MoveToCanonical moves a message into the Outlook folder behind a canonical
// one and returns its id there, which a move changes (copy plus delete). A
// message already in that folder is left alone and keeps its id. Delete lands
// in Deleted Items, as Outlook's own Delete key does.
func (c *Client) MoveToCanonical(ctx context.Context, messageID, folder string) (string, error) {
	dst, ok := WellKnownFolder(folder)
	if !ok {
		return "", fmt.Errorf("msgraph: %q is not a folder a message can be filed into", folder)
	}
	dstID, err := c.wellKnownFolderID(ctx, dst)
	if err != nil {
		return "", err
	}
	parentID, err := c.messageParentFolder(ctx, messageID)
	if err != nil {
		return "", err
	}
	if parentID == dstID {
		return messageID, nil
	}
	return c.move(ctx, messageID, dst)
}
