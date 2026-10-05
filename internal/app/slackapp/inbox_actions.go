package slackapp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/warmbly/warmbly/internal/app/aitools"
	"github.com/warmbly/warmbly/internal/models"
)

const (
	replyBodyMax     = 3000
	replySendTimeout = time.Minute
	labelInterested  = "Interested"
	labelNotInterest = "Not interested"
)

// replyMeta travels in the reply modal's private_metadata.
type replyMeta struct {
	ThreadID string `json:"t"`
}

// inboxTarget is a clicker allowed to work one mirrored conversation.
type inboxTarget struct {
	a       *actor
	mapping *models.SlackInboxThread
}

// resolveInbox checks the clicker is linked, holds inbox access, and that the
// conversation is mirrored for their own organization.
func (s *Service) resolveInbox(ctx context.Context, p *interaction, threadID string) *inboxTarget {
	if strings.TrimSpace(threadID) == "" {
		return nil
	}
	a := s.requireLinked(ctx, p)
	if a == nil {
		return nil
	}
	if !a.member.Permissions.HasPermission(models.PermAccessUnibox) {
		s.whisper(ctx, a.token, p, "You need inbox access in Warmbly to do that.")
		return nil
	}
	mapping, err := s.repo.GetInboxThread(ctx, a.link.OrganizationID, threadID)
	if err != nil {
		return nil
	}
	if mapping == nil {
		s.whisper(ctx, a.token, p, "This conversation belongs to another Warmbly workspace or is no longer mirrored here.")
		return nil
	}
	return &inboxTarget{a: a, mapping: mapping}
}

// latestDraft is the newest draft for the conversation: one written in Slack,
// else the inbox agent's pending draft.
func (s *Service) latestDraft(ctx context.Context, orgID uuid.UUID, threadID string) string {
	if d := s.getDraft(ctx, orgID, threadID); d != "" {
		return d
	}
	if s.drafts == nil {
		return ""
	}
	list, err := s.drafts.ListPendingDrafts(ctx, orgID, 100)
	if err != nil {
		return ""
	}
	for _, d := range list {
		if d.ThreadID == threadID {
			return d.Body
		}
	}
	return ""
}

// openReplyModal shows the reply form, prefilled with the latest draft.
func (s *Service) openReplyModal(ctx context.Context, p *interaction, threadID string) {
	t := s.resolveInbox(ctx, p, threadID)
	if t == nil {
		return
	}
	if s.registry == nil || s.threads == nil {
		s.whisper(ctx, t.a.token, p, "Replying from Slack is not available on this Warmbly instance.")
		return
	}
	to, subject := "", ""
	if res, err := s.threads.GetByThread(ctx, t.a.link.OrganizationID, uuid.Nil, threadID, 1, ""); err == nil && res != nil && len(res.Data) > 0 {
		to, subject = senderAddress(res.Data[0].FromAddr), res.Data[0].Subject
	}
	draft := truncateRunes(s.latestDraft(ctx, t.a.link.OrganizationID, threadID), replyBodyMax)
	if err := s.client.OpenView(ctx, t.a.token, p.TriggerID, replyModal(threadID, to, subject, draft)); err != nil {
		log.Warn().Err(err).Msg("slack: opening the reply modal failed")
	}
}

func replyModal(threadID, to, subject, draft string) Block {
	meta, _ := json.Marshal(replyMeta{ThreadID: threadID})
	input := Block{"type": "plain_text_input", "action_id": "body", "multiline": true, "max_length": replyBodyMax}
	if draft != "" {
		input["initial_value"] = draft
	}
	head := "*To:* " + escapeMrkdwn(orDash(to)) + "\n*Subject:* " + escapeMrkdwn(orDash(replySubject(subject)))
	return Block{
		"type":             "modal",
		"callback_id":      CallbackInboxReplyModal,
		"private_metadata": string(meta),
		"title":            plainText("Reply"),
		"submit":           plainText("Send"),
		"close":            plainText("Cancel"),
		"blocks": []Block{
			sectionBlock(head),
			{"type": "input", "block_id": "body", "label": plainText("Message"), "element": input},
			contextBlock("Sends as you, from the mailbox already in this conversation."),
		},
	}
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func replySubject(subject string) string {
	subject = strings.TrimSpace(subject)
	if subject == "" || strings.HasPrefix(strings.ToLower(subject), "re:") {
		return subject
	}
	return "Re: " + subject
}

// submitReply sends through the inbox send tool as the clicker, so the
// permission gate, entitlement, suppression check and audit match the dashboard.
func (s *Service) submitReply(ctx context.Context, p *interaction) any {
	body := p.value("body")
	if body == "" {
		return viewErrors("body", "Write a message first.")
	}
	var meta replyMeta
	if json.Unmarshal([]byte(p.View.PrivateMetadata), &meta) != nil || meta.ThreadID == "" {
		return viewErrors("body", "This form expired. Open it again from the conversation.")
	}
	if s.registry == nil {
		return viewErrors("body", "Replying from Slack is not available on this Warmbly instance.")
	}
	a := s.resolveActor(ctx, p.teamID(), p.User.ID)
	if a == nil || (a.link == nil && !s.autoLink(ctx, a)) {
		return viewErrors("body", "Link your Warmbly account first: mention @Warmbly or message it, and follow the link it sends you.")
	}
	if !a.member.Permissions.HasPermission(models.PermAccessUnibox) {
		return viewErrors("body", "You need inbox access in Warmbly to reply.")
	}
	orgID := a.link.OrganizationID
	mapping, err := s.repo.GetInboxThread(ctx, orgID, meta.ThreadID)
	if err != nil || mapping == nil {
		return viewErrors("body", "This conversation belongs to another Warmbly workspace.")
	}
	// The modal closes now and the send runs once in the background: Slack
	// allows three seconds here, and a send that outlives an open form invites
	// a second submit.
	if !s.guard.first(ctx, "slack:reply:"+p.View.ID, time.Hour) {
		return viewClear()
	}
	args, _ := json.Marshal(map[string]string{"thread_id": meta.ThreadID, "body": body})
	inv, token := invocation(a.member, a.link), a.token
	userID, slackUser := a.link.UserID, a.userID
	s.spawn("slack_reply_send", replySendTimeout, func(ctx context.Context) {
		if _, err := s.registry.Call(withSlackSend(ctx), inv, "send_reply", args); err != nil {
			m := plainMessage("Your reply was not sent. " + sendErrorText(err))
			m.Channel, m.ThreadTS, m.User = mapping.ChannelID, mapping.ThreadTS, slackUser
			if perr := s.client.PostEphemeral(ctx, token, m); perr != nil {
				log.Warn().Err(perr).Msg("slack: reply failure notice failed")
			}
			return
		}
		s.guard.del(ctx, draftKey(orgID, meta.ThreadID))
		s.inbox.postSent(ctx, orgID, userID, meta.ThreadID, body, time.Now(), slackUser)
	})
	return viewClear()
}

func sendErrorText(err error) string {
	switch {
	case errors.Is(err, aitools.ErrToolForbidden):
		return "You don't have permission to send from the inbox."
	case errors.Is(err, aitools.ErrInvalidArgs):
		return "This conversation has no message to reply to."
	case errors.Is(err, context.DeadlineExceeded):
		return "Sending took too long. Check the conversation in Warmbly before trying again."
	}
	return truncateRunes(err.Error(), 300)
}

// inboxDraft asks the assistant, as the clicker, to draft a reply in the
// conversation's Slack thread.
func (s *Service) inboxDraft(ctx context.Context, p *interaction, threadID, actionTS string) {
	t := s.resolveInbox(ctx, p, threadID)
	if t == nil {
		return
	}
	if refusal := s.channelRefusal(ctx, t.a, t.mapping.ChannelID); refusal != "" {
		s.whisper(ctx, t.a.token, p, refusal)
		return
	}
	ref, _ := json.Marshal(threadID)
	text := inboxContext(threadID) + "Read this conversation with get_thread, then write a reply draft with the draft_reply tool using thread_id " +
		string(ref) + ". Do not send anything. Show me the draft."
	s.runTurn(ctx, agentTurn{
		conn: t.a.conn, token: t.a.token, link: t.a.link, inv: invocation(t.a.member, t.a.link),
		channel: t.mapping.ChannelID, threadTS: t.mapping.ThreadTS,
		messageID: "slack:inbox_draft:" + t.mapping.ID.String() + ":" + actionTS,
		text:      text, reassign: true, quotesOthers: true,
	})
}

// inboxInterest files the conversation under Interested or Not interested,
// the labels the dashboard's inbox uses, through set_thread_labels.
func (s *Service) inboxInterest(ctx context.Context, p *interaction, threadID string, interested bool) {
	t := s.resolveInbox(ctx, p, threadID)
	if t == nil {
		return
	}
	if s.registry == nil || s.labels == nil || s.threads == nil {
		s.whisper(ctx, t.a.token, p, "Labelling from Slack is not available on this Warmbly instance.")
		return
	}
	orgID := t.a.link.OrganizationID
	want, drop := labelInterested, labelNotInterest
	if !interested {
		want, drop = drop, want
	}
	wantID, err := s.labels.EnsureCategory(ctx, orgID, want)
	if err != nil {
		s.whisper(ctx, t.a.token, p, errGenericAnswer)
		return
	}
	dropID, err := s.labels.EnsureCategory(ctx, orgID, drop)
	if err != nil {
		s.whisper(ctx, t.a.token, p, errGenericAnswer)
		return
	}
	current, err := s.threads.ListThreadLabels(ctx, orgID, threadID)
	if err != nil {
		s.whisper(ctx, t.a.token, p, errGenericAnswer)
		return
	}
	ids := []string{}
	for _, l := range current {
		if l.ID != wantID && l.ID != dropID {
			ids = append(ids, l.ID.String())
		}
	}
	ids = append(ids, wantID.String())
	args, _ := json.Marshal(map[string]any{"thread_id": threadID, "category_ids": ids})
	if _, err := s.registry.Call(ctx, invocation(t.a.member, t.a.link), "set_thread_labels", args); err != nil {
		s.whisper(ctx, t.a.token, p, sendErrorText(err))
		return
	}
	s.setParentState(ctx, t.a.token, p, t.mapping, "Marked *"+want+"* by <@"+p.User.ID+">")
}

// inboxAssign records who is handling the conversation. The unified inbox has
// no assignee, so this lives on the Slack card only.
func (s *Service) inboxAssign(ctx context.Context, p *interaction, threadID string) {
	t := s.resolveInbox(ctx, p, threadID)
	if t == nil {
		return
	}
	s.setParentState(ctx, t.a.token, p, t.mapping, "Handled by <@"+p.User.ID+">")
}

// setParentState rewrites the state line of the conversation's parent card.
func (s *Service) setParentState(ctx context.Context, token string, p *interaction, mapping *models.SlackInboxThread, line string) {
	if p.Container.MessageTS != mapping.ThreadTS || len(p.Message.Blocks) == 0 {
		s.say(ctx, token, mapping.ChannelID, mapping.ThreadTS, line)
		return
	}
	updated := withStateLine(p.Message.Blocks, line)
	if err := s.client.UpdateMessage(ctx, token, Message{
		Channel: mapping.ChannelID, TS: mapping.ThreadTS, Text: p.Message.Text, Blocks: updated,
	}); err != nil {
		log.Warn().Err(err).Msg("slack: updating the inbox card failed")
	}
}

// withStateLine replaces the card's state block, or adds one above its actions.
func withStateLine(in []Block, line string) []Block {
	state := contextBlock(line)
	state["block_id"] = blockInboxState
	out := make([]Block, 0, len(in)+1)
	placed := false
	for _, b := range in {
		switch b["block_id"] {
		case blockInboxState:
			if !placed {
				out = append(out, state)
				placed = true
			}
			continue
		case blockInboxActions:
			if !placed {
				out = append(out, state)
				placed = true
			}
		}
		out = append(out, b)
	}
	if !placed {
		out = append(out, state)
	}
	return out
}
