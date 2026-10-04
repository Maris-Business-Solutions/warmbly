package integration

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/models"
)

// SlackBotScopes are the bot scopes the Warmbly Slack app requests. The app
// manifest (internal/app/slackapp) lists the same set.
var SlackBotScopes = []string{
	"app_mentions:read",
	"assistant:write",
	"channels:history",
	"channels:read",
	"chat:write",
	"chat:write.public",
	"commands",
	"groups:history",
	"groups:read",
	"im:history",
	"im:read",
	"im:write",
	"mpim:history",
	"users:read",
	"users:read.email",
}

// Config keys SlackSettings owns inside config_capabilities.
const (
	slackKeyChannel           = "channel"
	slackKeyRoutes            = "routes"
	slackKeyAssistantDisabled = "assistant_disabled"
	slackKeyAssistantDMOnly   = "assistant_dm_only"
	slackKeyInboxChannel      = "inbox_channel"
	slackKeyInboxScope        = "inbox_scope"
)

var errSlackConnectionNotFound = errors.New("slack connection not found")

func usableSlack(c models.IntegrationConnection) bool {
	return c.Provider == models.IntegrationSlack &&
		(c.Status == models.IntegrationStatusConnected || c.Status == models.IntegrationStatusDegraded)
}

// SlackConnection returns the org's usable Slack connection (oldest first),
// or nil when the workspace has none.
func (s *service) SlackConnection(ctx context.Context, orgID uuid.UUID) (*models.IntegrationConnection, error) {
	conns, err := s.repo.ListConnections(ctx, orgID)
	if err != nil {
		return nil, err
	}
	var pick *models.IntegrationConnection
	for i := range conns {
		if !usableSlack(conns[i]) {
			continue
		}
		if pick == nil || conns[i].CreatedAt.Before(pick.CreatedAt) {
			c := conns[i]
			pick = &c
		}
	}
	return pick, nil
}

// SlackConnectionsForTeam lists the usable connections installed into a Slack
// team, across organizations, oldest first.
func (s *service) SlackConnectionsForTeam(ctx context.Context, teamID string) ([]models.IntegrationConnection, error) {
	conns, err := s.repo.ListConnectionsByExternalAccount(ctx, models.IntegrationSlack, teamID)
	if err != nil {
		return nil, err
	}
	out := make([]models.IntegrationConnection, 0, len(conns))
	for _, c := range conns {
		if usableSlack(c) {
			out = append(out, c)
		}
	}
	return out, nil
}

// SlackBotToken opens the bot token of a Slack connection owned by orgID.
func (s *service) SlackBotToken(ctx context.Context, orgID, connID uuid.UUID) (string, error) {
	sec, err := s.repo.GetConnectionSecrets(ctx, connID)
	if err != nil {
		return "", err
	}
	if sec == nil || sec.Conn.OrganizationID != orgID || sec.Conn.Provider != models.IntegrationSlack {
		return "", errSlackConnectionNotFound
	}
	return s.accessTokenFor(ctx, sec)
}

// SlackDefaultChannel is the legacy default channel for a connection with no
// SlackSettings.Channel: display fields, then a Slack automation's channel.
func (s *service) SlackDefaultChannel(ctx context.Context, orgID uuid.UUID, conn *models.IntegrationConnection) string {
	if conn == nil || conn.OrganizationID != orgID {
		return ""
	}
	return s.slackChannelFor(ctx, orgID, *conn)
}

// UpdateSlackSettings writes the Slack keys of config_capabilities and keeps
// every other key the connection holds.
func (s *service) UpdateSlackSettings(ctx context.Context, orgID, connID uuid.UUID, settings models.SlackSettings) (*models.IntegrationConnection, error) {
	conn, err := s.repo.GetConnectionByID(ctx, orgID, connID)
	if err != nil {
		return nil, err
	}
	if conn == nil || conn.Provider != models.IntegrationSlack {
		return nil, errSlackConnectionNotFound
	}
	cc := map[string]any{}
	if len(conn.ConfigCapabilities) > 0 {
		if err := json.Unmarshal(conn.ConfigCapabilities, &cc); err != nil {
			return nil, err
		}
	}
	setOrDelete := func(key string, v any, empty bool) {
		if empty {
			delete(cc, key)
			return
		}
		cc[key] = v
	}
	setOrDelete(slackKeyChannel, settings.Channel, settings.Channel == "")
	setOrDelete(slackKeyRoutes, settings.Routes, len(settings.Routes) == 0)
	setOrDelete(slackKeyAssistantDisabled, true, !settings.AssistantDisabled)
	setOrDelete(slackKeyAssistantDMOnly, true, !settings.AssistantDMOnly)
	setOrDelete(slackKeyInboxChannel, settings.InboxChannel, settings.InboxChannel == "")
	setOrDelete(slackKeyInboxScope, settings.InboxScope, settings.InboxScope == "")
	raw, err := json.Marshal(cc)
	if err != nil {
		return nil, err
	}
	dir := conn.SyncDirection
	if dir == "" {
		dir = "push"
	}
	if err := s.repo.UpdateConnectionConfig(ctx, orgID, connID, raw, dir); err != nil {
		return nil, err
	}
	return s.repo.GetConnectionByID(ctx, orgID, connID)
}

// MarkSlackTeamRevoked flags every connection installed into teamID (after an
// uninstall or a revoked token) and returns their ids.
func (s *service) MarkSlackTeamRevoked(ctx context.Context, teamID string, status models.IntegrationStatus, detail string) ([]uuid.UUID, error) {
	conns, err := s.repo.ListConnectionsByExternalAccount(ctx, models.IntegrationSlack, teamID)
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(conns))
	for _, c := range conns {
		if err := s.repo.SetConnectionStatus(ctx, c.ID, status, models.IntegrationHealthDown, detail); err != nil {
			return ids, err
		}
		ids = append(ids, c.ID)
	}
	return ids, nil
}

func (s *service) SlackOAuthConfigured() bool {
	return s.oauth.Configured(models.IntegrationSlack)
}

func (s *service) SlackOAuthRedirectURL() string {
	return s.oauth.RedirectURL()
}
