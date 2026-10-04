package slackapp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
)

// linkCodeLen is the base64url length of a 32-byte code.
const linkCodeLen = 43

func hashLinkCode(code string) []byte {
	sum := sha256.Sum256([]byte(code))
	return sum[:]
}

// mintLinkURL stores a fresh single-use code for a Slack member and returns
// the dashboard URL that redeems it ("" when the instance has no APP_URL).
func (s *Service) mintLinkURL(ctx context.Context, conn *models.IntegrationConnection, teamID, slackUserID string) string {
	u, _ := s.mintLink(ctx, conn, teamID, slackUserID)
	return u
}

// mintLink is mintLinkURL that also returns the code, to hold a question under.
func (s *Service) mintLink(ctx context.Context, conn *models.IntegrationConnection, teamID, slackUserID string) (string, string) {
	if appURL("/") == "" {
		return "", ""
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", ""
	}
	code := base64.RawURLEncoding.EncodeToString(raw)
	err := s.repo.CreateLinkCode(ctx, hashLinkCode(code), models.SlackLinkCode{
		OrganizationID: conn.OrganizationID,
		ConnectionID:   conn.ID,
		SlackTeamID:    teamID,
		SlackUserID:    slackUserID,
		ExpiresAt:      time.Now().Add(linkCodeTTL),
	})
	if err != nil {
		log.Warn().Err(err).Msg("slack: storing a link code failed")
		return "", ""
	}
	return appURL("/app/slack/link?code=" + url.QueryEscape(code)), code
}

// LinkPreview is GET /v1/integrations/slack/link/:code.
type LinkPreview struct {
	OrganizationID   uuid.UUID `json:"organization_id"`
	OrganizationName string    `json:"organization_name"`
	IsMember         bool      `json:"is_member"`
	models.SlackLinkPreview
}

func validCode(code string) bool {
	if len(code) != linkCodeLen {
		return false
	}
	_, err := base64.RawURLEncoding.DecodeString(code)
	return err == nil
}

// PreviewLink describes a pending code to the signed-in user before they confirm.
func (s *Service) PreviewLink(ctx context.Context, userID uuid.UUID, code string) (*LinkPreview, *errx.Error) {
	code = strings.TrimSpace(code)
	if !validCode(code) {
		return nil, ErrSlackLinkInvalid
	}
	c, err := s.repo.PreviewLinkCode(ctx, hashLinkCode(code))
	if err != nil {
		return nil, errx.InternalError()
	}
	if c == nil {
		return nil, ErrSlackLinkInvalid
	}
	out := &LinkPreview{
		OrganizationID: c.OrganizationID,
		SlackLinkPreview: models.SlackLinkPreview{
			SlackTeamID: c.SlackTeamID,
			SlackUserID: c.SlackUserID,
			ExpiresAt:   c.ExpiresAt,
		},
	}
	if org, xerr := s.orgs.Get(ctx, c.OrganizationID); xerr == nil && org != nil {
		out.OrganizationName = org.Name
	}
	if m, xerr := s.orgs.GetMembership(ctx, c.OrganizationID, userID); xerr == nil && m != nil && m.AcceptedAt != nil {
		out.IsMember = true
	}
	if conns, err := s.integ.ListConnections(ctx, c.OrganizationID); err == nil {
		for _, conn := range conns {
			if conn.ID == c.ConnectionID {
				out.SlackTeamName = conn.ExternalAccountName
			}
		}
	}
	return out, nil
}

// ConfirmLink redeems a code for the signed-in user, who must be an accepted
// member of the code's workspace. The code is spent in the same transaction.
func (s *Service) ConfirmLink(ctx context.Context, userID uuid.UUID, code string) (*models.SlackUserLink, *errx.Error) {
	code = strings.TrimSpace(code)
	if !validCode(code) {
		return nil, ErrSlackLinkInvalid
	}
	link, err := s.repo.ConsumeLinkCode(ctx, hashLinkCode(code), userID)
	switch {
	case errors.Is(err, repository.ErrSlackLinkCodeInvalid):
		return nil, ErrSlackLinkInvalid
	case errors.Is(err, repository.ErrSlackLinkNotMember):
		return nil, errx.New(errx.Forbidden, "You are not a member of the Warmbly workspace this link belongs to.")
	case err != nil || link == nil:
		return nil, errx.InternalError()
	}
	l := *link
	s.spawn("link_confirmation", agentRunTimeout, func(ctx context.Context) {
		q := s.takeAsk(ctx, l.OrganizationID, code)
		s.sendLinkConfirmation(ctx, &l, q != nil)
		if q != nil {
			s.resumeAsk(ctx, &l, q)
		}
	})
	return link, nil
}

// SlackInstalled implements integration.SlackInstallHook: the member who
// connected Slack is linked to the Slack account that approved the install.
func (s *Service) SlackInstalled(ctx context.Context, conn *models.IntegrationConnection, slackUserID string, userID uuid.UUID) {
	if conn == nil || slackUserID == "" || conn.ExternalAccountID == "" {
		return
	}
	link, err := s.repo.LinkInstaller(ctx, conn.OrganizationID, conn.ID, conn.ExternalAccountID, slackUserID, userID)
	if err != nil {
		log.Warn().Err(err).Msg("slack: linking the installer failed")
		return
	}
	if link == nil {
		return
	}
	if s.audit != nil {
		s.audit.LogAction(ctx, link.OrganizationID, userID, models.AuditActionCreate, models.AuditEntityIntegration,
			&link.ConnectionID, "", "Slack", nil, map[string]string{"slack_link": "linked_on_install"})
	}
	l := *link
	s.spawn("link_confirmation", shortTaskTime, func(ctx context.Context) {
		s.sendLinkConfirmation(ctx, &l, false)
	})
}

// resumeAsk answers the question that was held while its author linked.
func (s *Service) resumeAsk(ctx context.Context, link *models.SlackUserLink, q *ask) {
	a := s.resolveActor(ctx, link.SlackTeamID, link.SlackUserID)
	if a == nil || a.link == nil {
		return
	}
	s.handleAsk(ctx, a, *q)
}

func (s *Service) sendLinkConfirmation(ctx context.Context, link *models.SlackUserLink, resuming bool) {
	token, err := s.integ.SlackBotToken(ctx, link.OrganizationID, link.ConnectionID)
	if err != nil {
		return
	}
	dm, err := s.client.OpenDM(ctx, token, link.SlackUserID)
	if err != nil {
		return
	}
	orgName := "your Warmbly workspace"
	if org, xerr := s.orgs.Get(ctx, link.OrganizationID); xerr == nil && org != nil && org.Name != "" {
		orgName = "*" + escapeMrkdwn(org.Name) + "*"
	}
	text := "You're linked to " + orgName + ". Ask me anything about your outreach right here, or mention @Warmbly in any channel I'm in."
	if resuming {
		text = "You're linked to " + orgName + ". I'm answering your question now."
	}
	if _, err := s.client.PostMessage(ctx, token, Message{Channel: dm, Text: "You're linked to Warmbly", Blocks: blocks(sectionBlock(text))}); err != nil {
		log.Warn().Err(err).Msg("slack: link confirmation DM failed")
	}
}

// UpdateMyLink toggles the caller's DM notifications.
func (s *Service) UpdateMyLink(ctx context.Context, orgID, userID uuid.UUID, dm bool) (*models.SlackUserLink, *errx.Error) {
	link, err := s.repo.SetLinkDMNotifications(ctx, orgID, userID, dm)
	if err != nil {
		return nil, errx.InternalError()
	}
	if link == nil {
		return nil, errx.NewWithIdentifier(errx.NotFound, "slack_not_linked", "You have not linked a Slack account in this workspace.")
	}
	return link, nil
}

// UnlinkMine removes the caller's own link; removing nothing is not an error.
func (s *Service) UnlinkMine(ctx context.Context, orgID, userID uuid.UUID) *errx.Error {
	if _, err := s.repo.DeleteLinkForUser(ctx, orgID, userID); err != nil {
		return errx.InternalError()
	}
	return nil
}

// RemoveLink removes any member's link in the org.
func (s *Service) RemoveLink(ctx context.Context, orgID, linkID uuid.UUID) (*models.SlackUserLink, *errx.Error) {
	link, err := s.repo.DeleteLink(ctx, orgID, linkID)
	if err != nil {
		return nil, errx.InternalError()
	}
	if link == nil {
		return nil, errx.New(errx.NotFound, "Slack link not found.")
	}
	return link, nil
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
