package slackapp

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/warmbly/warmbly/internal/models"
)

// eventEnvelope is an Events API delivery.
type eventEnvelope struct {
	Type           string          `json:"type"`
	Challenge      string          `json:"challenge"`
	TeamID         string          `json:"team_id"`
	APIAppID       string          `json:"api_app_id"`
	EventID        string          `json:"event_id"`
	Event          json.RawMessage `json:"event"`
	Authorizations []struct {
		TeamID string `json:"team_id"`
		UserID string `json:"user_id"`
		IsBot  bool   `json:"is_bot"`
	} `json:"authorizations"`
	IsExtSharedChannel bool `json:"is_ext_shared_channel"`
}

// innerEvent is the part of any subscribed event the app reads.
type innerEvent struct {
	Type            string `json:"type"`
	Subtype         string `json:"subtype"`
	User            string `json:"user"`
	BotID           string `json:"bot_id"`
	Text            string `json:"text"`
	TS              string `json:"ts"`
	ThreadTS        string `json:"thread_ts"`
	Channel         string `json:"channel"`
	ChannelType     string `json:"channel_type"`
	Tab             string `json:"tab"`
	AssistantThread struct {
		UserID    string `json:"user_id"`
		ChannelID string `json:"channel_id"`
		ThreadTS  string `json:"thread_ts"`
	} `json:"assistant_thread"`
	Tokens struct {
		Bot []string `json:"bot"`
	} `json:"tokens"`
}

// HandleEvents answers a verified Events API delivery. url_verification is
// echoed; everything else is acknowledged and handled in the background,
// once per event id.
func (s *Service) HandleEvents(ctx context.Context, body []byte) (any, error) {
	var env eventEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, err
	}
	switch env.Type {
	case "url_verification":
		return map[string]string{"challenge": env.Challenge}, nil
	case "event_callback":
	default:
		return nil, nil
	}
	if env.EventID != "" && !s.guard.first(ctx, "slack:event:"+env.EventID, eventDedupeTTL) {
		return nil, nil
	}
	s.spawn("slack_event", agentRunTimeout, func(ctx context.Context) {
		s.dispatchEvent(ctx, &env)
	})
	return nil, nil
}

func (s *Service) dispatchEvent(ctx context.Context, env *eventEnvelope) {
	var ev innerEvent
	if err := json.Unmarshal(env.Event, &ev); err != nil {
		return
	}
	for _, a := range env.Authorizations {
		if a.IsBot && a.UserID != "" && a.TeamID == env.TeamID {
			s.botIDs.Store(env.TeamID, a.UserID)
		}
	}
	switch ev.Type {
	case "app_mention":
		s.onUserMessage(ctx, env, &ev, true)
	case "message":
		if ev.BotID != "" || ev.User == "" || ev.ChannelType == "mpim" {
			return
		}
		switch ev.Subtype {
		case "", "thread_broadcast", "file_share":
			s.onUserMessage(ctx, env, &ev, false)
		}
	case "assistant_thread_started":
		s.onAssistantThreadStarted(ctx, env, &ev)
	case "assistant_thread_context_changed":
		// The assistant reads context from the thread itself; nothing is kept.
	case "app_home_opened":
		if ev.Tab == "home" {
			s.publishHome(ctx, env.TeamID, env.APIAppID, ev.User)
		}
	case "app_uninstalled":
		s.revokeTeam(ctx, env.TeamID, models.IntegrationStatusDisconnected, "The Warmbly app was removed from Slack.")
	case "tokens_revoked":
		if len(ev.Tokens.Bot) > 0 {
			s.revokeTeam(ctx, env.TeamID, models.IntegrationStatusReauthRequired, "Slack revoked the bot token: reconnect Slack.")
		}
	}
}

// revokeTeam flags every connection installed into the team and drops links.
func (s *Service) revokeTeam(ctx context.Context, teamID string, status models.IntegrationStatus, detail string) {
	ids, err := s.integ.MarkSlackTeamRevoked(ctx, teamID, status, detail)
	if err != nil {
		log.Warn().Err(err).Str("team_id", teamID).Msg("slack: marking connections revoked failed")
	}
	if err := s.repo.DeleteLinksForConnections(ctx, ids); err != nil {
		log.Warn().Err(err).Str("team_id", teamID).Msg("slack: dropping links failed")
	}
}

// actor is a Slack member resolved to a connection and, when linked and still
// a member, to the Warmbly member they act as.
type actor struct {
	teamID  string
	userID  string
	conn    *models.IntegrationConnection
	token   string
	link    *models.SlackUserLink
	member  *models.OrganizationMember
	unknown bool // membership could not be read; try again later
	gone    bool // was linked, but is no longer a member
}

// resolveActor returns nil when the team has no usable connection.
func (s *Service) resolveActor(ctx context.Context, teamID, slackUserID string) *actor {
	conns, err := s.integ.SlackConnectionsForTeam(ctx, teamID)
	if err != nil || len(conns) == 0 {
		return nil
	}
	link, err := s.repo.GetLinkBySlackUser(ctx, teamID, slackUserID)
	if err != nil {
		return nil
	}
	conn, linked := resolveLink(conns, link)
	token, err := s.integ.SlackBotToken(ctx, conn.OrganizationID, conn.ID)
	if err != nil {
		return nil
	}
	a := &actor{teamID: teamID, userID: slackUserID, conn: conn, token: token}
	if linked {
		m, st := s.membership(ctx, link)
		switch st {
		case memberOK:
			a.link, a.member = link, m
		case memberGone:
			a.gone = true
		case memberUnknown:
			a.unknown = true
		}
	}
	return a
}

// tell answers one member: in the DM thread, or ephemerally in a channel.
func (s *Service) tell(ctx context.Context, token, channel, threadTS, user string, dm bool, m Message) {
	m.Channel, m.ThreadTS = channel, threadTS
	var err error
	if dm {
		_, err = s.client.PostMessage(ctx, token, m)
	} else {
		m.User = user
		err = s.client.PostEphemeral(ctx, token, m)
	}
	if err != nil {
		log.Warn().Err(err).Msg("slack: reply to member failed")
	}
}

// promptLink tells an unlinked member how to link, or why they must again.
func (s *Service) promptLink(ctx context.Context, a *actor, channel, threadTS string, dm bool) {
	lead := ""
	if a.gone {
		lead = "Your Warmbly membership changed, so this Slack account is no longer linked. Link it again to keep using Warmbly here."
	}
	if a.unknown {
		s.tell(ctx, a.token, channel, threadTS, a.userID, dm, plainMessage("I couldn't check your Warmbly account just now. Please try again in a moment."))
		return
	}
	s.tell(ctx, a.token, channel, threadTS, a.userID, dm, linkPrompt(s.mintLinkURL(ctx, a.conn, a.teamID, a.userID), lead))
}

func inboxContext(uniboxThreadID string) string {
	b, _ := json.Marshal(uniboxThreadID)
	return "This Slack thread is the team's discussion of the email conversation with thread_id " + string(b) +
		" in the Warmbly unified inbox. When the question is about that conversation, read it with get_thread first. Never send email unless I explicitly ask.\n\n"
}

// onUserMessage routes a DM, a mention, or a follow-up in a thread the
// author owns to the assistant.
func (s *Service) onUserMessage(ctx context.Context, env *eventEnvelope, ev *innerEvent, mention bool) {
	a := s.resolveActor(ctx, env.TeamID, ev.User)
	if a == nil {
		return
	}
	botID := s.botUserID(ctx, env.TeamID, a.token)
	if botID != "" && ev.User == botID {
		return
	}
	dm := ev.ChannelType == "im" || (ev.ChannelType == "" && strings.HasPrefix(ev.Channel, "D"))
	inThread := ev.ThreadTS != "" && ev.ThreadTS != ev.TS
	threadTS := ev.ThreadTS
	if threadTS == "" {
		threadTS = ev.TS
	}

	// An inbox thread is team discussion: only a mention reaches the assistant.
	var inbox *models.SlackInboxThread
	if !dm && inThread {
		inbox, _ = s.repo.GetInboxThreadBySlack(ctx, a.conn.ID, ev.Channel, threadTS)
		if inbox != nil && !mention {
			return
		}
	}

	row, err := s.repo.GetAgentThread(ctx, a.conn.ID, ev.Channel, threadTS)
	if err != nil {
		return
	}
	facts := routeFacts{
		DM: dm, Mention: mention, MentionsBot: mentionsUser(ev.Text, botID), InThread: inThread,
		ExtShared: env.IsExtSharedChannel, Settings: settingsFrom(a.conn),
	}
	owner := row != nil && a.link != nil && row.OrganizationID == a.link.OrganizationID && row.UserID == a.link.UserID
	if row != nil && inbox == nil {
		facts.ThreadMapped, facts.OwnerIsAuthor = true, owner
	}
	route := decideRoute(facts)
	switch route {
	case routeIgnore:
		return
	case routeAgent:
	default:
		s.tell(ctx, a.token, ev.Channel, threadTS, ev.User, dm, plainMessage(refusalText(route)))
		return
	}
	if a.link == nil {
		s.promptLink(ctx, a, ev.Channel, threadTS, dm)
		return
	}
	text := stripBotMention(ev.Text, botID)
	if text == "" {
		s.tell(ctx, a.token, ev.Channel, threadTS, ev.User, dm, plainMessage("Ask me anything about your campaigns, replies, contacts or mailboxes."))
		return
	}
	prefix := ""
	if inThread && (row == nil || !owner) {
		prefix = s.threadContext(ctx, a.token, ev.Channel, threadTS, ev.TS)
	}
	if inbox != nil {
		prefix = inboxContext(inbox.UniboxThreadID) + prefix
	}
	s.runTurn(ctx, agentTurn{
		conn: a.conn, token: a.token, link: a.link, inv: invocation(a.member, a.link),
		channel: ev.Channel, threadTS: threadTS, messageID: "slack:" + env.EventID,
		text: prefix + text, dm: dm, reassign: inbox != nil,
	})
}

// onAssistantThreadStarted seeds the assistant pane with suggested prompts.
func (s *Service) onAssistantThreadStarted(ctx context.Context, env *eventEnvelope, ev *innerEvent) {
	at := ev.AssistantThread
	if at.ChannelID == "" || at.ThreadTS == "" {
		return
	}
	a := s.resolveActor(ctx, env.TeamID, at.UserID)
	if a == nil {
		return
	}
	if err := s.client.SetSuggestedPrompts(ctx, a.token, at.ChannelID, at.ThreadTS, "Try asking", suggestedPrompts); err != nil {
		log.Warn().Err(err).Msg("slack: suggested prompts failed")
	}
	if a.link == nil {
		s.promptLink(ctx, a, at.ChannelID, at.ThreadTS, true)
	}
}
