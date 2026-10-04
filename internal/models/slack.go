package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// SlackUserLink binds a Slack member to a Warmbly member of the workspace that
// installed the Slack app. Anything the bot does for that Slack member runs as
// UserID with the member's current organization permissions.
type SlackUserLink struct {
	ID              uuid.UUID `json:"id"`
	OrganizationID  uuid.UUID `json:"organization_id"`
	ConnectionID    uuid.UUID `json:"connection_id"`
	SlackTeamID     string    `json:"slack_team_id"`
	SlackUserID     string    `json:"slack_user_id"`
	UserID          uuid.UUID `json:"user_id"`
	DMNotifications bool      `json:"dm_notifications"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`

	// Joined for display.
	UserName  string `json:"user_name,omitempty"`
	UserEmail string `json:"user_email,omitempty"`
}

// SlackLinkCode is a pending link minted by the bot and redeemed in the
// dashboard. Only the hash of the code is stored.
type SlackLinkCode struct {
	OrganizationID uuid.UUID
	ConnectionID   uuid.UUID
	SlackTeamID    string
	SlackUserID    string
	ExpiresAt      time.Time
}

// SlackAgentThread maps a Slack thread to the assistant session answering it.
type SlackAgentThread struct {
	ID                uuid.UUID `json:"id"`
	OrganizationID    uuid.UUID `json:"organization_id"`
	ConnectionID      uuid.UUID `json:"connection_id"`
	ChannelID         string    `json:"channel_id"`
	ThreadTS          string    `json:"thread_ts"`
	SessionID         uuid.UUID `json:"session_id"`
	UserID            uuid.UUID `json:"user_id"`
	ApprovalMessageTS string    `json:"approval_message_ts,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// SlackSettings is the non-secret Slack configuration kept in the connection's
// config_capabilities. Channel values are Slack channel ids (C…/G…); a legacy
// "#name" still posts for public channels.
type SlackSettings struct {
	// Channel is the default channel for notifications (pre-existing key).
	Channel string `json:"channel,omitempty"`
	// Routes sends a notification category to its own channel; a category
	// absent here uses Channel.
	Routes map[NotificationCategory]string `json:"routes,omitempty"`
	// AssistantDisabled turns the assistant off for the whole Slack workspace.
	AssistantDisabled bool `json:"assistant_disabled,omitempty"`
	// AssistantDMOnly keeps the assistant out of channels: it answers in DMs
	// and the assistant pane only.
	AssistantDMOnly bool `json:"assistant_dm_only,omitempty"`
	// InboxChannel mirrors unified-inbox arrivals into this channel, one
	// Slack thread per conversation. Empty turns the inbox off.
	InboxChannel string `json:"inbox_channel,omitempty"`
	// InboxScope is SlackInboxScopeReplies (the default) or SlackInboxScopeAll.
	InboxScope string `json:"inbox_scope,omitempty"`
}

// Slack inbox scopes: human replies only, or every inbound message.
const (
	SlackInboxScopeReplies = "replies"
	SlackInboxScopeAll     = "all"
)

// SlackInboxThread maps an inbox conversation to its Slack thread.
type SlackInboxThread struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	ConnectionID   uuid.UUID `json:"connection_id"`
	ChannelID      string    `json:"channel_id"`
	ThreadTS       string    `json:"thread_ts"`
	UniboxThreadID string    `json:"unibox_thread_id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// SlackStatus is GET /v1/integrations/slack/status for the dashboard panel.
type SlackStatus struct {
	// AppConfigured: the instance has Slack OAuth credentials.
	AppConfigured bool `json:"app_configured"`
	// InteractiveConfigured: the instance also has a signing secret, so events,
	// the assistant, slash commands and buttons work.
	InteractiveConfigured bool `json:"interactive_configured"`
	// Connection is the workspace's Slack connection, nil when not connected.
	Connection *IntegrationConnection `json:"connection,omitempty"`
	// MissingScopes lists bot scopes the install predates; reconnecting grants them.
	MissingScopes []string        `json:"missing_scopes"`
	Settings      SlackSettings   `json:"settings"`
	MyLink        *SlackUserLink  `json:"my_link,omitempty"`
	Links         []SlackUserLink `json:"links"`
}

// SlackChannel is one entry of the channel picker.
type SlackChannel struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsPrivate bool   `json:"is_private"`
	IsMember  bool   `json:"is_member"`
}

// SlackLinkPreview is GET /v1/integrations/slack/link/:code, shown before the
// member confirms.
type SlackLinkPreview struct {
	SlackTeamID   string `json:"slack_team_id"`
	SlackTeamName string `json:"slack_team_name"`
	SlackUserID   string `json:"slack_user_id"`
	// SlackUserName and SlackUserAvatar describe the Slack account being
	// linked, read from Slack when the preview is built; either may be empty.
	SlackUserName   string `json:"slack_user_name"`
	SlackUserAvatar string `json:"slack_user_avatar"`
	// EmailMatches: the Slack account's email is the caller's Warmbly email,
	// which confirming requires.
	EmailMatches bool      `json:"email_matches"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// SlackLinkEmailMatches reports whether a Slack profile email and a Warmbly
// account email name the same address. An empty side never matches.
func SlackLinkEmailMatches(slackEmail, userEmail string) bool {
	a, b := strings.TrimSpace(slackEmail), strings.TrimSpace(userEmail)
	return a != "" && b != "" && strings.Contains(a, "@") && strings.EqualFold(a, b)
}
