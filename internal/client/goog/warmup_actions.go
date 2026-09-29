package goog

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/googleapi"
)

// FileWarmup takes a warmup message out of the mailbox's visible views and,
// when labelName is set, files it under that label instead.
//
// Gmail has no folders: a view IS a system label, so "moving" a message means
// removing the label the view is built from. INBOX for the copy a mailbox
// received, SENT for the copy of what it sent. Adding a label and stopping
// there is what this used to do, and it is why warmup mail sat in the
// customer's inbox wearing a tidy label (both stay listed in All Mail, which
// is the point: nothing is deleted).
//
// An empty labelName is the archive placement: the message keeps no label of
// its own and is reachable only through All Mail and search.
func (c *Client) FileWarmup(ctx context.Context, messageID, labelName string) error {
	if c.srv == nil {
		return fmt.Errorf("gmail service not initialized")
	}

	var add []string
	if labelName != "" {
		labelID, err := c.getOrCreateLabel(ctx, labelName)
		if err != nil {
			return fmt.Errorf("failed to get/create label %q: %w", labelName, err)
		}
		add = []string{labelID}
	}

	// Removing a label a message does not carry is a no-op, so one request
	// covers both directions without asking which one this is.
	remove := []string{Inbox, Sent}
	if c.sentLabelStuck.Load() {
		remove = []string{Inbox}
	}

	err := c.modifyLabels(ctx, messageID, add, remove)
	if err == nil {
		return nil
	}
	if len(remove) == 1 || !isLabelRefusal(err) {
		return fmt.Errorf("failed to file warmup message: %w", err)
	}
	// Gmail refused to take the message out of SENT. Remember it for this
	// mailbox so later sends cost one request rather than two, and file the
	// copy under the label anyway: the Sent view still lists it, but the
	// warmup label is what the owner can read the folder by.
	c.sentLabelStuck.Store(true)
	if err := c.modifyLabels(ctx, messageID, add, []string{Inbox}); err != nil {
		return fmt.Errorf("failed to file warmup message: %w", err)
	}
	return nil
}

// modifyLabels issues one label transition, omitting an empty side rather than
// sending an empty array.
func (c *Client) modifyLabels(ctx context.Context, messageID string, add, remove []string) error {
	req := &gmail.ModifyMessageRequest{}
	if len(add) > 0 {
		req.AddLabelIds = add
	}
	if len(remove) > 0 {
		req.RemoveLabelIds = remove
	}
	_, err := c.srv.Users.Messages.Modify("me", messageID, req).Context(ctx).Do()
	return err
}

// isLabelRefusal reports whether Gmail rejected the request because of a label
// it will not let the caller change, as opposed to anything about the message.
func isLabelRefusal(err error) bool {
	var gerr *googleapi.Error
	if !errors.As(err, &gerr) {
		return false
	}
	return gerr.Code == 400 && strings.Contains(strings.ToLower(gerr.Message), "label")
}

// MarkAsRead marks a message as read by removing the UNREAD label
func (c *Client) MarkAsRead(ctx context.Context, messageID string) error {
	if c.srv == nil {
		return fmt.Errorf("gmail service not initialized")
	}

	_, err := c.srv.Users.Messages.Modify("me", messageID, &gmail.ModifyMessageRequest{
		RemoveLabelIds: []string{Unread},
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to mark as read: %w", err)
	}

	return nil
}

// RescueFromSpam is the "not spam" signal warmup exists to generate: the SPAM
// label comes off, which is exactly what pressing "Not spam" does.
//
// Where the message goes next is the mailbox's placement, not automatically the
// inbox. Putting it back there unconditionally is the second way warmup mail
// ended up in front of the customer, and it undid the foldering that had just
// run. toInbox is the "leave warmup in the inbox" placement; otherwise the
// message lands under labelName (or nowhere, for archive).
func (c *Client) RescueFromSpam(ctx context.Context, messageID, labelName string, toInbox bool) error {
	if c.srv == nil {
		return fmt.Errorf("gmail service not initialized")
	}

	var add []string
	if toInbox {
		add = []string{Inbox}
	} else if labelName != "" {
		labelID, err := c.getOrCreateLabel(ctx, labelName)
		if err != nil {
			return fmt.Errorf("failed to get/create label %q: %w", labelName, err)
		}
		add = []string{labelID}
	}

	if err := c.modifyLabels(ctx, messageID, add, []string{Spam}); err != nil {
		return fmt.Errorf("failed to remove from spam: %w", err)
	}
	return nil
}

// AddStar stars a message by adding the STARRED system label. Distinct from
// MarkImportant (the IMPORTANT label): a star is a deliberate, visible positive
// signal, so providers weight it as genuine engagement.
func (c *Client) AddStar(ctx context.Context, messageID string) error {
	if c.srv == nil {
		return fmt.Errorf("gmail service not initialized")
	}

	_, err := c.srv.Users.Messages.Modify("me", messageID, &gmail.ModifyMessageRequest{
		AddLabelIds: []string{Starred},
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to add star: %w", err)
	}

	return nil
}

// MarkImportant marks a message as important
func (c *Client) MarkImportant(ctx context.Context, messageID string) error {
	if c.srv == nil {
		return fmt.Errorf("gmail service not initialized")
	}

	_, err := c.srv.Users.Messages.Modify("me", messageID, &gmail.ModifyMessageRequest{
		AddLabelIds: []string{Important},
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to mark important: %w", err)
	}

	return nil
}

// getOrCreateLabel finds an existing label or creates a new one. The id is
// remembered for the mailbox's lifetime: warmup files two copies of every
// message it sends and the label list is one request each time.
func (c *Client) getOrCreateLabel(ctx context.Context, labelName string) (string, error) {
	if id, ok := c.labelID(labelName); ok {
		return id, nil
	}

	// List existing labels
	resp, err := c.srv.Users.Labels.List("me").Context(ctx).Do()
	if err != nil {
		return "", err
	}

	for _, label := range resp.Labels {
		if label.Name == labelName {
			c.cacheLabel(labelName, label.Id)
			return label.Id, nil
		}
	}

	// Create new label
	newLabel, err := c.srv.Users.Labels.Create("me", &gmail.Label{
		Name:                  labelName,
		LabelListVisibility:   "labelShow",
		MessageListVisibility: "show",
	}).Context(ctx).Do()
	if err != nil {
		return "", err
	}

	c.cacheLabel(labelName, newLabel.Id)
	return newLabel.Id, nil
}

func (c *Client) labelID(name string) (string, bool) {
	c.labelMu.Lock()
	defer c.labelMu.Unlock()
	id, ok := c.labelIDs[name]
	return id, ok
}

func (c *Client) cacheLabel(name, id string) {
	c.labelMu.Lock()
	defer c.labelMu.Unlock()
	if c.labelIDs == nil {
		c.labelIDs = map[string]string{}
	}
	c.labelIDs[name] = id
}

// SetSeen flips the read state of many messages in one call.
//
// batchModify rather than a modify per message: the unibox's "mark all as
// read" is one press over a folder, and a per-message call there is hundreds
// of round trips against a per-user rate limit. Gmail accepts up to 1000 ids
// per request; the caller chunks.
func (c *Client) SetSeen(ctx context.Context, messageIDs []string, seen bool) error {
	if c.srv == nil {
		return fmt.Errorf("gmail service not initialized")
	}
	if len(messageIDs) == 0 {
		return nil
	}

	req := &gmail.BatchModifyMessagesRequest{Ids: messageIDs}
	if seen {
		req.RemoveLabelIds = []string{Unread}
	} else {
		req.AddLabelIds = []string{Unread}
	}
	if err := c.srv.Users.Messages.BatchModify("me", req).Context(ctx).Do(); err != nil {
		return fmt.Errorf("failed to set read state on %d messages: %w", len(messageIDs), err)
	}
	return nil
}

// Trash moves a message to Gmail's Trash, which Gmail empties on its own
// after thirty days. This is the deletion the retention window asks for: the
// modify scope the mailbox was connected with allows trashing and not the
// permanent delete, and Trash is also what a person pressing Delete gets.
func (c *Client) Trash(ctx context.Context, messageID string) error {
	if c.srv == nil {
		return fmt.Errorf("gmail service not initialized")
	}
	_, err := c.srv.Users.Messages.Trash("me", messageID).Context(ctx).Do()
	if err != nil {
		var gerr *googleapi.Error
		if errors.As(err, &gerr) && gerr.Code == 404 {
			// Already gone, which is the state being asked for.
			return nil
		}
		return fmt.Errorf("failed to trash message: %w", err)
	}
	return nil
}

// FindByRFCMessageID returns the Gmail id of the message carrying the RFC
// 5322 Message-ID, or "" when the mailbox does not have it. Trash and Spam
// are excluded by Gmail's search by default, which is right here: a message
// already in either needs nothing more from the retention sweep.
func (c *Client) FindByRFCMessageID(ctx context.Context, rfcMessageID string) (string, error) {
	rfcMessageID = strings.Trim(strings.TrimSpace(rfcMessageID), "<>")
	if rfcMessageID == "" || c.srv == nil {
		return "", nil
	}
	ids, _, err := c.ListMessages(ctx, "rfc822msgid:"+rfcMessageID, "", 1)
	if err != nil {
		return "", err
	}
	if len(ids) == 0 {
		return "", nil
	}
	return ids[0], nil
}

// LocateRFCMessageID reports whether the mailbox still holds the message
// carrying this RFC 5322 Message-ID, and whether every copy it holds is in
// Trash. Spam is searched too: a message filed there was moved, not deleted.
func (c *Client) LocateRFCMessageID(ctx context.Context, rfcMessageID string) (found, trashed bool, err error) {
	rfcMessageID = strings.Trim(strings.TrimSpace(rfcMessageID), "<>")
	if rfcMessageID == "" || c.srv == nil {
		return false, false, errors.New("gmail: no message id to look up")
	}
	resp, err := c.srv.Users.Messages.List("me").Q("rfc822msgid:" + rfcMessageID).
		IncludeSpamTrash(true).MaxResults(10).Context(ctx).Do()
	if err != nil {
		return false, false, HandleError(err)
	}
	for _, m := range resp.Messages {
		if m == nil || m.Id == "" {
			continue
		}
		msg, err := c.srv.Users.Messages.Get("me", m.Id).Format("minimal").Context(ctx).Do()
		if err != nil {
			var gerr *googleapi.Error
			if errors.As(err, &gerr) && gerr.Code == 404 {
				continue
			}
			return false, false, HandleError(err)
		}
		found = true
		if !slices.Contains(msg.LabelIds, Trash) {
			return true, false, nil
		}
	}
	return found, found, nil
}
