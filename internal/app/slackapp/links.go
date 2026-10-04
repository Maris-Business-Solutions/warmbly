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
	"github.com/warmbly/warmbly/internal/pkg/displayname"
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
	if appURL("/") == "" {
		return ""
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return ""
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
		return ""
	}
	return appURL("/app/slack/link?code=" + url.QueryEscape(code))
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
	prof, xerr := s.slackProfile(ctx, c)
	if xerr != nil {
		return nil, xerr
	}
	out := &LinkPreview{
		OrganizationID: c.OrganizationID,
		SlackLinkPreview: models.SlackLinkPreview{
			SlackTeamID:     c.SlackTeamID,
			SlackUserID:     c.SlackUserID,
			SlackUserName:   prof.Name,
			SlackUserAvatar: prof.Avatar,
			ExpiresAt:       c.ExpiresAt,
		},
	}
	if s.users != nil {
		if u, err := s.users.GetUser(ctx, userID); err == nil && u != nil {
			out.EmailMatches = models.SlackLinkEmailMatches(prof.Email, u.Email)
		}
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

// linkProfile is the Slack account a code names, as Slack reports it now.
type linkProfile struct {
	Name   string
	Email  string
	Avatar string
}

// profileFrom keeps a name safe to show, an https avatar, and the email only
// when Slack has not marked it unconfirmed.
func profileFrom(u *slackUser) linkProfile {
	var p linkProfile
	if u == nil || u.Deleted || u.IsBot {
		return p
	}
	for _, n := range []string{u.Profile.DisplayName, u.Profile.RealName} {
		if p.Name = displayname.Clean(n, displayname.Person); p.Name != "" {
			break
		}
	}
	if u.IsEmailConfirmed == nil || *u.IsEmailConfirmed {
		p.Email = strings.TrimSpace(u.Profile.Email)
	}
	if v, err := url.Parse(u.Profile.Image72); err == nil && v.Scheme == "https" && v.Host != "" {
		p.Avatar = v.String()
	}
	return p
}

// slackProfile reads the code's Slack account. An install without the users
// scopes yields an empty profile, which no Warmbly email matches.
func (s *Service) slackProfile(ctx context.Context, c *models.SlackLinkCode) (linkProfile, *errx.Error) {
	token, err := s.integ.SlackBotToken(ctx, c.OrganizationID, c.ConnectionID)
	if err != nil {
		return linkProfile{}, ErrSlackLinkInvalid
	}
	u, err := s.client.UserInfo(ctx, token, c.SlackUserID)
	switch {
	case IsAPIError(err, "missing_scope", "user_not_found"):
		return linkProfile{}, nil
	case err != nil:
		return linkProfile{}, errx.NewPublic(errx.ServiceUnavailable, "Slack could not be reached to check this account. Try again in a moment.")
	}
	return profileFrom(u), nil
}

// ConfirmLink redeems a code for the signed-in user, who must be an accepted
// member of the code's workspace whose email is the Slack account's email.
// The code is spent in the same transaction.
func (s *Service) ConfirmLink(ctx context.Context, userID uuid.UUID, code string) (*models.SlackUserLink, *errx.Error) {
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
	prof, xerr := s.slackProfile(ctx, c)
	if xerr != nil {
		return nil, xerr
	}
	link, err := s.repo.ConsumeLinkCode(ctx, hashLinkCode(code), userID, prof.Email)
	switch {
	case errors.Is(err, repository.ErrSlackLinkCodeInvalid):
		return nil, ErrSlackLinkInvalid
	case errors.Is(err, repository.ErrSlackLinkNotMember):
		return nil, errx.New(errx.Forbidden, "You are not a member of the Warmbly workspace this link belongs to.")
	case errors.Is(err, repository.ErrSlackLinkEmailMismatch):
		return nil, ErrSlackLinkEmailMismatch
	case err != nil || link == nil:
		return nil, errx.InternalError()
	}
	l := *link
	s.spawn("link_confirmation", shortTaskTime, func(ctx context.Context) {
		s.sendLinkConfirmation(ctx, &l)
	})
	return link, nil
}

func (s *Service) sendLinkConfirmation(ctx context.Context, link *models.SlackUserLink) {
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
	text := "You're linked to " + orgName + ". Ask me anything about your outreach right here, mention me in a channel, or type `/warmbly help`."
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
