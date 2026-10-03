package slackapp

import (
	"context"
	"net/url"
	"strings"

	"github.com/warmbly/warmbly/internal/models"
)

const helpText = "*Warmbly in Slack*\n" +
	"• `/warmbly <question>`: ask about your campaigns, replies, contacts or mailboxes. I answer in your DM.\n" +
	"• Message me directly, or mention @Warmbly in a channel thread.\n" +
	"• *Ask Warmbly about this* in any message's menu starts a thread about it.\n" +
	"• `/warmbly link` links your Warmbly account, `/warmbly unlink` removes the link."

func ephemeral(m Message) map[string]any {
	out := map[string]any{"response_type": "ephemeral", "text": m.Text}
	if len(m.Blocks) > 0 {
		out["blocks"] = m.Blocks
	}
	return out
}

// HandleCommand answers a verified /warmbly request within Slack's 3 seconds;
// a question is answered afterwards in the member's DM.
func (s *Service) HandleCommand(ctx context.Context, body []byte) (any, error) {
	form, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, errBadPayload
	}
	teamID, userID := form.Get("team_id"), form.Get("user_id")
	text := strings.TrimSpace(form.Get("text"))
	word, _, _ := strings.Cut(text, " ")
	word = strings.ToLower(word)

	if word == "" || word == "help" {
		return ephemeral(plainMessage(helpText)), nil
	}
	a := s.resolveActor(ctx, teamID, userID)
	if a == nil {
		return ephemeral(plainMessage("Warmbly is not connected to this Slack workspace. A Warmbly admin can connect it in Integrations.")), nil
	}
	if a.unknown {
		return ephemeral(plainMessage("I couldn't check your Warmbly account just now. Please try again in a moment.")), nil
	}

	switch word {
	case "link":
		if a.link != nil {
			return ephemeral(plainMessage(s.linkedLine(ctx, a) + " Type `/warmbly unlink` to remove the link.")), nil
		}
		return ephemeral(linkPrompt(s.mintLinkURL(ctx, a.conn, teamID, userID), "")), nil
	case "unlink":
		if a.link == nil {
			return ephemeral(plainMessage("This Slack account is not linked to Warmbly.")), nil
		}
		if _, err := s.repo.DeleteLinkBySlackUser(ctx, a.link.OrganizationID, teamID, userID); err != nil {
			return ephemeral(plainMessage(errGenericAnswer)), nil
		}
		s.auditUnlink(ctx, a.link)
		return ephemeral(plainMessage("Unlinked. Link again any time with `/warmbly link`.")), nil
	}

	if a.link == nil {
		lead := ""
		if a.gone {
			lead = "Your Warmbly membership changed, so this Slack account is no longer linked. Link it again to keep using Warmbly here."
		}
		return ephemeral(linkPrompt(s.mintLinkURL(ctx, a.conn, teamID, userID), lead)), nil
	}
	if settingsFrom(a.conn).AssistantDisabled {
		return ephemeral(plainMessage(refusalText(routeRefuseDisabled))), nil
	}
	question := text
	trigger := form.Get("trigger_id")
	s.spawn("slack_command_question", agentRunTimeout, func(ctx context.Context) {
		dm, ts, err := s.startDMThread(ctx, a, "*You asked:* "+escapeMrkdwn(truncateRunes(question, 2000)))
		if err != nil {
			return
		}
		s.runTurn(ctx, agentTurn{
			conn: a.conn, token: a.token, link: a.link, inv: invocation(a.member, a.link),
			channel: dm, threadTS: ts, messageID: "slack:command:" + trigger, text: question, dm: true,
		})
	})
	return ephemeral(plainMessage("Answering in your DM with Warmbly.")), nil
}

func (s *Service) linkedLine(ctx context.Context, a *actor) string {
	org := "your Warmbly workspace"
	if o, xerr := s.orgs.Get(ctx, a.link.OrganizationID); xerr == nil && o != nil && o.Name != "" {
		org = "*" + escapeMrkdwn(o.Name) + "*"
	}
	who := ""
	if a.link.UserName != "" {
		who = " as *" + escapeMrkdwn(a.link.UserName) + "*"
	}
	return "You're linked to " + org + who + "."
}

// auditUnlink records a link removed from Slack on the audit spine.
func (s *Service) auditUnlink(ctx context.Context, link *models.SlackUserLink) {
	if s.audit == nil || link == nil {
		return
	}
	s.audit.LogAction(ctx, link.OrganizationID, link.UserID, models.AuditActionDelete, models.AuditEntityIntegration,
		&link.ConnectionID, "", "Slack", nil, map[string]string{"slack_link": "removed_from_slack"})
}
