package slackapp

import (
	"strings"

	"github.com/warmbly/warmbly/internal/app/integration"
)

// Callback and command ids shared by the manifest and the handlers.
const (
	SlashCommand             = "/warmbly"
	CallbackMessageShortcut  = "ask_warmbly_about_message"
	ActionApprove            = "agent_approve"
	ActionDeny               = "agent_deny"
	ActionAlwaysAllow        = "agent_always"
	ActionDraftReply         = "notif_draft_reply"
	ActionHomeUnlink         = "home_unlink"
	ActionOpenURL            = "open_url"
	pathSlackEvents          = "/api/v1/integrations/slack/events"
	pathSlackInteractivity   = "/api/v1/integrations/slack/interactivity"
	pathSlackCommands        = "/api/v1/integrations/slack/commands"
	pathIntegrationsCallback = "/integrations/oauth/callback"
)

// BotEvents are the events the app subscribes to.
var BotEvents = []string{
	"app_mention",
	"message.im",
	"message.channels",
	"message.groups",
	"assistant_thread_started",
	"assistant_thread_context_changed",
	"app_home_opened",
	"app_uninstalled",
	"tokens_revoked",
}

// suggestedPrompts seed the assistant pane and a new assistant thread.
var suggestedPrompts = []SuggestedPrompt{
	{Title: "Summarize today's replies", Message: "Summarize the replies my campaigns got today and flag anything that needs an answer."},
	{Title: "How are my campaigns doing?", Message: "How are my active campaigns performing this week?"},
	{Title: "Check mailbox health", Message: "Are any of my mailboxes unhealthy or throttled right now?"},
	{Title: "Find a contact", Message: "Find the contact I emailed most recently and show their status."},
}

// RequestURLs are the URLs a Slack app for this instance points at.
type RequestURLs struct {
	Events        string `json:"events"`
	Interactivity string `json:"interactivity"`
	Commands      string `json:"commands"`
	OAuthRedirect string `json:"oauth_redirect"`
}

// requestURLs derives the Slack request URLs from the backend's public origin.
func requestURLs(backendURL, oauthRedirect string) RequestURLs {
	base := strings.TrimRight(backendURL, "/")
	if oauthRedirect == "" {
		oauthRedirect = base + pathIntegrationsCallback
	}
	return RequestURLs{
		Events:        base + pathSlackEvents,
		Interactivity: base + pathSlackInteractivity,
		Commands:      base + pathSlackCommands,
		OAuthRedirect: oauthRedirect,
	}
}

// Manifest is the Slack app manifest for an instance whose API answers at
// backendURL. deploy/slack/manifest.json is this with the placeholder host.
func Manifest(backendURL, oauthRedirect string) map[string]any {
	u := requestURLs(backendURL, oauthRedirect)
	prompts := make([]any, 0, len(suggestedPrompts))
	for _, p := range suggestedPrompts {
		prompts = append(prompts, map[string]any{"title": p.Title, "message": p.Message})
	}
	scopes := make([]any, 0, len(integration.SlackBotScopes))
	for _, s := range integration.SlackBotScopes {
		scopes = append(scopes, s)
	}
	events := make([]any, 0, len(BotEvents))
	for _, e := range BotEvents {
		events = append(events, e)
	}
	return map[string]any{
		"display_information": map[string]any{
			"name":             "Warmbly",
			"description":      "Ask Warmbly about your outreach, work inbox replies and get notified, all from Slack.",
			"background_color": "#0c4a6e",
		},
		"features": map[string]any{
			"app_home": map[string]any{
				"home_tab_enabled":               true,
				"messages_tab_enabled":           true,
				"messages_tab_read_only_enabled": false,
			},
			"bot_user": map[string]any{
				"display_name":  "Warmbly",
				"always_online": true,
			},
			"assistant_view": map[string]any{
				"assistant_description": "Ask about your campaigns, replies, contacts and mailboxes. Warmbly answers as you, with your workspace permissions.",
				"suggested_prompts":     prompts,
			},
			"shortcuts": []any{
				map[string]any{
					"name":        "Ask Warmbly about this",
					"type":        "message",
					"callback_id": CallbackMessageShortcut,
					"description": "Start a Warmbly assistant thread about this message",
				},
			},
			"slash_commands": []any{
				map[string]any{
					"command":       SlashCommand,
					"url":           u.Commands,
					"description":   "Ask Warmbly a question or link your account",
					"usage_hint":    "[question] | link | unlink | help",
					"should_escape": false,
				},
			},
		},
		"oauth_config": map[string]any{
			"redirect_urls": []any{u.OAuthRedirect},
			"scopes":        map[string]any{"bot": scopes},
		},
		"settings": map[string]any{
			"event_subscriptions": map[string]any{
				"request_url": u.Events,
				"bot_events":  events,
			},
			"interactivity": map[string]any{
				"is_enabled":  true,
				"request_url": u.Interactivity,
			},
			"org_deploy_enabled":     false,
			"socket_mode_enabled":    false,
			"token_rotation_enabled": false,
		},
	}
}
