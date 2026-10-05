// Mirror of internal/models/slack.go: the Slack app panel, channel picker and
// the member link flow.

import type { IntegrationConnection } from "./Integration";
import type Pagination from "../Pagination";

export interface SlackUserLink {
    id: string;
    organization_id: string;
    connection_id: string;
    slack_team_id: string;
    slack_user_id: string;
    user_id: string;
    dm_notifications: boolean;
    created_at: Date;
    updated_at: Date;
    user_name?: string;
    user_email?: string;
}

// Non-secret Slack configuration. Channel values are Slack channel ids
// (C… / G…); a legacy "#name" still posts for public channels.
export interface SlackSettings {
    channel?: string;
    // Category key -> channel; a category absent here uses `channel`.
    routes?: Record<string, string>;
    assistant_disabled?: boolean;
    assistant_dm_only?: boolean;
    // Inbox in Slack: the channel new unified-inbox replies thread into; empty is off.
    inbox_channel?: string;
    inbox_scope?: SlackInboxScope;
}

// "replies" (the default) skips auto-replies, out-of-office and bounces.
export type SlackInboxScope = "replies" | "all";

export interface SlackStatus {
    app_configured: boolean;
    interactive_configured: boolean;
    connection?: IntegrationConnection | null;
    missing_scopes: string[] | null;
    settings: SlackSettings;
    my_link?: SlackUserLink | null;
    links: SlackUserLink[] | null;
}

export interface SlackChannel {
    id: string;
    name: string;
    is_private: boolean;
    is_member: boolean;
}

export interface SlackChannelList {
    data: SlackChannel[];
    pagination: Pick<Pagination, "next_cursor" | "has_more">;
}

export interface SlackLinkPreview {
    organization_id: string;
    organization_name: string;
    is_member: boolean;
    slack_team_id: string;
    slack_team_name: string;
    slack_user_id: string;
    // The Slack account's name and avatar as Slack reports them; either may be empty.
    slack_user_name: string;
    slack_user_avatar: string;
    // The signed-in Warmbly account the link would be made for.
    user_email: string;
    // The Slack account's email is the signed-in user's Warmbly email, so one click links.
    email_matches: boolean;
    // Sign in with Slack can confirm the link when the emails differ.
    verify_available: boolean;
    expires_at: Date;
}
