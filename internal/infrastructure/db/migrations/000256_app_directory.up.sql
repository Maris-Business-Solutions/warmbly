-- Community app directory: an OAuth app's public listing. A published listing
-- is reachable by its link; it enters discovery when an operator features it
-- or enough workspaces use it. A hidden listing is reachable nowhere.
CREATE TABLE IF NOT EXISTS app_directory_listings (
    application_id   uuid PRIMARY KEY REFERENCES oauth_applications (id) ON DELETE CASCADE,
    organization_id  uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    slug             text NOT NULL UNIQUE
        CHECK (slug ~ '^[a-z0-9][a-z0-9-]{1,46}[a-z0-9]$'),
    tagline          text NOT NULL CHECK (char_length(tagline) BETWEEN 1 AND 120),
    description      text NOT NULL DEFAULT '' CHECK (char_length(description) <= 2000),
    category         text NOT NULL
        CHECK (category IN ('crm', 'automation', 'notifications', 'meetings', 'data', 'verification', 'ai', 'other')),
    install_url      text NOT NULL,
    support_url      text NOT NULL DEFAULT '',
    privacy_url      text NOT NULL DEFAULT '',
    status           text NOT NULL DEFAULT 'published'
        CHECK (status IN ('published', 'featured', 'hidden')),
    status_note      text NOT NULL DEFAULT '',
    status_by        uuid REFERENCES users (id) ON DELETE SET NULL,
    status_at        timestamptz,
    submitted_at     timestamptz NOT NULL DEFAULT now(),
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_app_directory_listings_org
    ON app_directory_listings (organization_id);

CREATE INDEX IF NOT EXISTS idx_app_directory_listings_status
    ON app_directory_listings (status);

-- An operator's suspension, kept apart from the owner's own enable/disable so
-- the owner cannot lift it. A suspended app authorizes nothing and its tokens
-- stop working.
ALTER TABLE oauth_applications
    ADD COLUMN IF NOT EXISTS suspended_at     timestamptz,
    ADD COLUMN IF NOT EXISTS suspended_reason text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS suspended_by     uuid REFERENCES users (id) ON DELETE SET NULL;

-- Workspaces and people an operator has stopped from registering or publishing
-- OAuth apps. Exactly one of organization_id and user_id is set.
CREATE TABLE IF NOT EXISTS oauth_developer_blocks (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id  uuid REFERENCES organizations (id) ON DELETE CASCADE,
    user_id          uuid REFERENCES users (id) ON DELETE CASCADE,
    reason           text NOT NULL DEFAULT '',
    blocked_by       uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    CHECK ((organization_id IS NULL) <> (user_id IS NULL))
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_oauth_developer_blocks_org
    ON oauth_developer_blocks (organization_id) WHERE organization_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_oauth_developer_blocks_user
    ON oauth_developer_blocks (user_id) WHERE user_id IS NOT NULL;
