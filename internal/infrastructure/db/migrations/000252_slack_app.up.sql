-- Slack app: account links, assistant threads and the inbox mirror.

-- A Slack member linked to a Warmbly member. The assistant runs as user_id with
-- that member's current permissions; one Slack member talks to one workspace.
CREATE TABLE IF NOT EXISTS slack_user_links (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id   uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    connection_id     uuid NOT NULL REFERENCES integration_connections (id) ON DELETE CASCADE,
    slack_team_id     text NOT NULL,
    slack_user_id     text NOT NULL,
    user_id           uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    dm_notifications  boolean NOT NULL DEFAULT false,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    UNIQUE (slack_team_id, slack_user_id),
    UNIQUE (connection_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_slack_user_links_org_user
    ON slack_user_links (organization_id, user_id);

-- Single-use link codes, stored hashed. Minted by the bot, redeemed by a
-- signed-in member of the connection's workspace.
CREATE TABLE IF NOT EXISTS slack_link_codes (
    code_hash        bytea PRIMARY KEY,
    organization_id  uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    connection_id    uuid NOT NULL REFERENCES integration_connections (id) ON DELETE CASCADE,
    slack_team_id    text NOT NULL,
    slack_user_id    text NOT NULL,
    expires_at       timestamptz NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_slack_link_codes_expires ON slack_link_codes (expires_at);

-- A Slack thread the assistant answers in, bound to the agent session that holds
-- its transcript and to the member who started it.
CREATE TABLE IF NOT EXISTS slack_agent_threads (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id    uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    connection_id      uuid NOT NULL REFERENCES integration_connections (id) ON DELETE CASCADE,
    channel_id         text NOT NULL,
    thread_ts          text NOT NULL,
    session_id         uuid NOT NULL REFERENCES agent_sessions (id) ON DELETE CASCADE,
    user_id            uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    approval_message_ts text,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    UNIQUE (connection_id, channel_id, thread_ts)
);

CREATE INDEX IF NOT EXISTS idx_slack_agent_threads_session ON slack_agent_threads (session_id);

-- A unified-inbox conversation mirrored into the workspace's Slack inbox
-- channel: one Slack thread per conversation.
CREATE TABLE IF NOT EXISTS slack_inbox_threads (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id   uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    connection_id     uuid NOT NULL REFERENCES integration_connections (id) ON DELETE CASCADE,
    channel_id        text NOT NULL,
    thread_ts         text NOT NULL,
    unibox_thread_id  text NOT NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, unibox_thread_id)
);

CREATE INDEX IF NOT EXISTS idx_slack_inbox_threads_slack
    ON slack_inbox_threads (connection_id, channel_id, thread_ts);
