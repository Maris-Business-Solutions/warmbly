package slackapp

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// interaction is an interactivity payload (block_actions, view_submission,
// message_action).
type interaction struct {
	Type       string `json:"type"`
	TriggerID  string `json:"trigger_id"`
	CallbackID string `json:"callback_id"`
	APIAppID   string `json:"api_app_id"`
	Team       struct {
		ID string `json:"id"`
	} `json:"team"`
	User struct {
		ID     string `json:"id"`
		TeamID string `json:"team_id"`
	} `json:"user"`
	Channel struct {
		ID string `json:"id"`
	} `json:"channel"`
	Container struct {
		Type      string `json:"type"`
		ChannelID string `json:"channel_id"`
		MessageTS string `json:"message_ts"`
	} `json:"container"`
	Message struct {
		TS       string  `json:"ts"`
		ThreadTS string  `json:"thread_ts"`
		Text     string  `json:"text"`
		User     string  `json:"user"`
		Blocks   []Block `json:"blocks"`
	} `json:"message"`
	Actions []struct {
		ActionID string `json:"action_id"`
		Value    string `json:"value"`
		ActionTS string `json:"action_ts"`
	} `json:"actions"`
	View struct {
		ID              string `json:"id"`
		CallbackID      string `json:"callback_id"`
		PrivateMetadata string `json:"private_metadata"`
		State           struct {
			Values map[string]map[string]struct {
				Value string `json:"value"`
			} `json:"values"`
		} `json:"state"`
	} `json:"view"`
}

func (p *interaction) teamID() string {
	if p.Team.ID != "" {
		return p.Team.ID
	}
	return p.User.TeamID
}

func (p *interaction) channelID() string {
	if p.Container.ChannelID != "" {
		return p.Container.ChannelID
	}
	return p.Channel.ID
}

// value reads one plain_text_input from a submitted view.
func (p *interaction) value(block string) string {
	return strings.TrimSpace(p.View.State.Values[block][block].Value)
}

var errBadPayload = errors.New("slack: unreadable interactivity payload")

// HandleInteractivity answers a verified interactivity request. A view
// submission is answered synchronously (Slack needs errors or "clear");
// everything else is acknowledged and handled in the background.
func (s *Service) HandleInteractivity(ctx context.Context, body []byte) (any, error) {
	form, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, errBadPayload
	}
	var p interaction
	if err := json.Unmarshal([]byte(form.Get("payload")), &p); err != nil {
		return nil, errBadPayload
	}
	switch p.Type {
	case "view_submission":
		return s.handleViewSubmission(ctx, &p), nil
	case "block_actions":
		if len(p.Actions) == 0 {
			return nil, nil
		}
		s.spawn("slack_action", agentRunTimeout, func(ctx context.Context) {
			s.handleBlockAction(ctx, &p)
		})
	case "message_action":
		if p.CallbackID == CallbackMessageShortcut {
			s.spawn("slack_message_shortcut", agentRunTimeout, func(ctx context.Context) {
				s.askAboutMessage(ctx, &p)
			})
		}
	}
	return nil, nil
}

func (s *Service) handleBlockAction(ctx context.Context, p *interaction) {
	act := p.Actions[0]
	switch {
	case act.ActionID == ActionApprove:
		s.handleApproval(ctx, p, act.Value, decisionApprove)
	case act.ActionID == ActionDeny:
		s.handleApproval(ctx, p, act.Value, decisionDeny)
	case act.ActionID == ActionAlwaysAllow:
		s.handleApproval(ctx, p, act.Value, decisionAlways)
	case act.ActionID == ActionDraftReply:
		s.handleNotificationDraft(ctx, p, act.Value, act.ActionTS)
	case act.ActionID == ActionInboxReply || act.ActionID == ActionInboxReview:
		s.openReplyModal(ctx, p, act.Value)
	case act.ActionID == ActionInboxDraft:
		s.inboxDraft(ctx, p, act.Value, act.ActionTS)
	case act.ActionID == ActionInboxInterested:
		s.inboxInterest(ctx, p, act.Value, true)
	case act.ActionID == ActionInboxNotInterested:
		s.inboxInterest(ctx, p, act.Value, false)
	case act.ActionID == ActionInboxAssign:
		s.inboxAssign(ctx, p, act.Value)
	case act.ActionID == ActionHomeUnlink:
		s.homeUnlink(ctx, p)
	}
}

// whisper answers the clicker privately where they clicked; in the App Home,
// where there is no channel, it does nothing.
func (s *Service) whisper(ctx context.Context, token string, p *interaction, text string) {
	channel := p.channelID()
	if channel == "" || token == "" {
		return
	}
	m := plainMessage(text)
	m.Channel, m.User = channel, p.User.ID
	if p.Message.ThreadTS != "" {
		m.ThreadTS = p.Message.ThreadTS
	}
	if err := s.client.PostEphemeral(ctx, token, m); err != nil {
		log.Warn().Err(err).Msg("slack: ephemeral reply failed")
	}
}

// requireLinked resolves the clicker and answers for them when they cannot act.
func (s *Service) requireLinked(ctx context.Context, p *interaction) *actor {
	a := s.resolveActor(ctx, p.teamID(), p.User.ID)
	if a == nil {
		return nil
	}
	if a.link == nil {
		channel := p.channelID()
		dm := strings.HasPrefix(channel, "D")
		if channel == "" {
			dmChannel, err := s.client.OpenDM(ctx, a.token, a.userID)
			if err != nil {
				return nil
			}
			channel, dm = dmChannel, true
		}
		s.promptLink(ctx, a, channel, p.Message.ThreadTS, dm)
		return nil
	}
	return a
}

// handleApproval resumes a paused run; only the session owner's linked Slack
// member may decide, and each card is decided once.
func (s *Service) handleApproval(ctx context.Context, p *interaction, value, decision string) {
	rowID, toolCallID, ok := parseApprovalValue(value)
	if !ok {
		return
	}
	a := s.requireLinked(ctx, p)
	if a == nil {
		return
	}
	row, err := s.repo.GetAgentThreadByID(ctx, a.link.OrganizationID, rowID)
	if err != nil {
		return
	}
	if row == nil || row.OrganizationID != a.link.OrganizationID || row.UserID != a.link.UserID {
		s.whisper(ctx, a.token, p, "Only the person who asked can approve this.")
		return
	}
	token, err := s.integ.SlackBotToken(ctx, row.OrganizationID, row.ConnectionID)
	if err != nil {
		return
	}
	cardTS := p.Container.MessageTS
	if row.ApprovalMessageTS == "" || row.ApprovalMessageTS != cardTS ||
		!s.guard.first(ctx, "slack:approval:"+row.ID.String()+":"+cardTS, time.Hour) {
		_ = s.client.UpdateMessage(ctx, token, Message{
			Channel: row.ChannelID, TS: cardTS, Text: "No longer waiting",
			Blocks: blocks(contextBlock("This request is no longer waiting for approval.")),
		})
		return
	}
	verb := map[string]string{decisionApprove: "Approved", decisionDeny: "Denied", decisionAlways: "Always allowed"}[decision]
	_ = s.client.UpdateMessage(ctx, token, Message{
		Channel: row.ChannelID, TS: cardTS, Text: verb,
		Blocks: blocks(contextBlock(verb + " by <@" + p.User.ID + ">")),
	})
	_ = s.repo.SetAgentThreadApproval(ctx, row.OrganizationID, row.ID, "")
	s.resumeTurn(ctx, token, row, invocation(a.member, a.link), toolCallID, decision)
}

// channelRefusal applies workspace settings and Slack Connect to an explicit
// request in a channel.
func (s *Service) channelRefusal(ctx context.Context, a *actor, channel string) string {
	dm := strings.HasPrefix(channel, "D")
	ext := false
	if !dm {
		if info, err := s.client.ConversationInfo(ctx, a.token, channel); err == nil {
			ext = info.IsExtShared
		}
	}
	r := decideRoute(routeFacts{DM: dm, Mention: true, ExtShared: ext, Settings: settingsFrom(a.conn)})
	if r == routeAgent {
		return ""
	}
	return refusalText(r)
}

// handleNotificationDraft starts the assistant under a reply notification for
// the clicker, to read the inbound thread and draft (never send) a reply.
func (s *Service) handleNotificationDraft(ctx context.Context, p *interaction, value, actionTS string) {
	var v draftReplyValue
	if json.Unmarshal([]byte(value), &v) != nil || (v.EmailID == "" && v.ThreadID == "") {
		return
	}
	a := s.requireLinked(ctx, p)
	if a == nil {
		return
	}
	if !s.conversationInOrg(ctx, a.link.OrganizationID, v) {
		s.whisper(ctx, a.token, p, "That conversation is not in your Warmbly workspace.")
		return
	}
	channel, ts := p.channelID(), p.Container.MessageTS
	if refusal := s.channelRefusal(ctx, a, channel); refusal != "" {
		s.whisper(ctx, a.token, p, refusal)
		return
	}
	ref, _ := json.Marshal(map[string]string{"thread_id": v.ThreadID, "message_id": v.EmailID})
	text := "A reply just arrived in the unified inbox (" + string(ref) + "). Read the conversation with get_thread, " +
		"then write a reply draft with the draft_reply tool. Do not send anything. Show me the draft."
	s.runTurn(ctx, agentTurn{
		conn: a.conn, token: a.token, link: a.link, inv: invocation(a.member, a.link),
		channel: channel, threadTS: ts, messageID: "slack:action:" + ts + ":" + actionTS,
		text: text, dm: strings.HasPrefix(channel, "D"), reassign: true,
	})
}

// conversationInOrg reports whether every message a draft card names is stored
// in orgID's mailboxes.
func (s *Service) conversationInOrg(ctx context.Context, orgID uuid.UUID, v draftReplyValue) bool {
	if s.threads == nil {
		return false
	}
	if v.ThreadID != "" {
		res, err := s.threads.GetByThread(ctx, orgID, uuid.Nil, v.ThreadID, 1, "")
		if err != nil || res == nil || len(res.Data) == 0 {
			return false
		}
	}
	if v.EmailID != "" {
		id, err := uuid.Parse(v.EmailID)
		if err != nil {
			return false
		}
		if e, _, err := s.threads.GetByIDForOrg(ctx, orgID, id); err != nil || e == nil {
			return false
		}
	}
	return true
}

// askAboutMessage starts the assistant about one message: in its thread, or
// in the member's DM when the bot cannot answer where the message is.
func (s *Service) askAboutMessage(ctx context.Context, p *interaction) {
	a := s.requireLinked(ctx, p)
	if a == nil {
		return
	}
	channel, msgTS := p.channelID(), p.Message.TS
	threadTS := p.Message.ThreadTS
	if threadTS == "" {
		threadTS = msgTS
	}
	st := settingsFrom(a.conn)
	if st.AssistantDisabled {
		s.whisper(ctx, a.token, p, refusalText(routeRefuseDisabled))
		return
	}
	info, err := s.client.ConversationInfo(ctx, a.token, channel)
	useDM := err != nil || info.IsIM || info.IsMpIM || (info.IsPrivate && !info.IsMember) || st.AssistantDMOnly
	if err == nil && info.IsExtShared {
		s.whisper(ctx, a.token, p, refusalText(routeRefuseExternal))
		return
	}

	quoted := formatThreadContext([]slackMessage{{User: p.Message.User, Text: p.Message.Text, TS: msgTS}}, "")
	if !useDM && p.Message.ThreadTS != "" {
		if c := s.threadContext(ctx, a.token, channel, threadTS, ""); c != "" {
			quoted = c
		}
	}
	question := quoted + "Help me with the quoted Slack message: summarize what it says and suggest what to do next in Warmbly."
	turn := agentTurn{
		conn: a.conn, token: a.token, link: a.link, inv: invocation(a.member, a.link),
		channel: channel, threadTS: threadTS, messageID: "slack:shortcut:" + p.TriggerID, text: question,
	}
	if useDM {
		dm, ts, err := s.startDMThread(ctx, a, "You asked about a message. Here is what I found.")
		if err != nil {
			return
		}
		turn.channel, turn.threadTS, turn.dm = dm, ts, true
	}
	s.runTurn(ctx, turn)
}

// startDMThread posts a header in the member's DM with the bot and returns
// its channel and ts, so a run can answer under it.
func (s *Service) startDMThread(ctx context.Context, a *actor, header string) (string, string, error) {
	dm, err := s.client.OpenDM(ctx, a.token, a.userID)
	if err != nil {
		return "", "", err
	}
	m := plainMessage(header)
	m.Channel = dm
	ts, err := s.client.PostMessage(ctx, a.token, m)
	if err != nil {
		return "", "", err
	}
	return dm, ts, nil
}

// handleViewSubmission answers a modal submit with errors or "clear".
func (s *Service) handleViewSubmission(ctx context.Context, p *interaction) any {
	switch p.View.CallbackID {
	case CallbackInboxReplyModal:
		return s.submitReply(ctx, p)
	}
	return nil
}

func viewErrors(block, msg string) map[string]any {
	return map[string]any{"response_action": "errors", "errors": map[string]string{block: msg}}
}

func viewClear() map[string]any {
	return map[string]any{"response_action": "clear"}
}

// homeUnlink removes the clicker's own link from the App Home.
func (s *Service) homeUnlink(ctx context.Context, p *interaction) {
	link, err := s.repo.GetLinkBySlackUser(ctx, p.teamID(), p.User.ID)
	if err != nil || link == nil {
		return
	}
	if _, err := s.repo.DeleteLinkBySlackUser(ctx, link.OrganizationID, link.SlackTeamID, link.SlackUserID); err != nil {
		return
	}
	s.auditUnlink(ctx, link)
	s.publishHome(ctx, p.teamID(), p.APIAppID, p.User.ID)
}
