package slackapp

import (
	"context"
	"net/url"
	"strings"

	"github.com/rs/zerolog/log"
)

// appRedirectURL opens the app's Messages tab, where the assistant lives.
func appRedirectURL(teamID, appID string) string {
	if appID == "" || teamID == "" {
		return ""
	}
	return "https://slack.com/app_redirect?app=" + url.QueryEscape(appID) + "&team=" + url.QueryEscape(teamID)
}

// publishHome renders the App Home for one Slack member.
func (s *Service) publishHome(ctx context.Context, teamID, appID, slackUserID string) {
	a := s.resolveActor(ctx, teamID, slackUserID)
	if a == nil {
		return
	}
	if err := s.client.PublishView(ctx, a.token, slackUserID, s.homeView(ctx, a, appID)); err != nil {
		log.Warn().Err(err).Msg("slack: publishing the home tab failed")
	}
}

func (s *Service) homeView(ctx context.Context, a *actor, appID string) Block {
	var bl []Block
	bl = append(bl, headerBlock("Warmbly"))
	open := urlButton("Open Warmbly", appURL("/app"))
	ask := urlButton("Ask Warmbly", appRedirectURL(a.teamID, appID))
	if a.link != nil {
		bl = append(bl, sectionBlock(s.linkedLine(ctx, a)))
		unlink := actionButton("Unlink", ActionHomeUnlink, "unlink", "danger")
		unlink["confirm"] = Block{
			"title":   plainText("Unlink Slack?"),
			"text":    plainText("Warmbly will stop answering you here until you link again."),
			"confirm": plainText("Unlink"),
			"deny":    plainText("Cancel"),
		}
		bl = append(bl, buttonsBlock(ask, open, unlink))
		dm := "off"
		if a.link.DMNotifications {
			dm = "on"
		}
		bl = append(bl, contextBlock("Notifications by DM are "+dm+". Change it in Warmbly under Integrations > Slack."))
	} else {
		lead := "Link your Warmbly account so I can answer as you, with your workspace permissions."
		if a.gone {
			lead = "Your Warmbly membership changed, so this Slack account is no longer linked. Link it again to keep using Warmbly here."
		}
		bl = append(bl, sectionBlock(lead), buttonsBlock(urlButton("Link your Warmbly account", s.mintLinkURL(ctx, a.conn, a.teamID, a.userID)), open))
	}
	bl = append(bl,
		Block{"type": "divider"},
		sectionBlock(strings.Join([]string{
			"*What you can do here*",
			"• Message me, or open the assistant, to ask about campaigns, replies, contacts and mailboxes.",
			"• Mention @Warmbly in a channel thread to bring me into the conversation.",
			"• Type `/warmbly` and a question from anywhere.",
			"• Use *Ask Warmbly about this* on any message.",
		}, "\n")),
	)
	if ch := settingsFrom(a.conn).InboxChannel; ch != "" {
		bl = append(bl, contextBlock("Inbox replies arrive in "+channelMention(ch)+", one thread per conversation."))
	}
	return Block{"type": "home", "blocks": blocks(bl...)}
}

// channelMention renders a channel setting as a Slack channel link.
func channelMention(ch string) string {
	if strings.HasPrefix(ch, "#") {
		return escapeMrkdwn(ch)
	}
	return "<#" + ch + ">"
}
