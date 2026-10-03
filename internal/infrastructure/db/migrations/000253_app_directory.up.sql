-- Community app directory: an OAuth app's public listing. A listing is reachable
-- by its link as soon as it is published and enters discovery only once an
-- operator has verified it.
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
    verification     text NOT NULL DEFAULT 'unverified'
        CHECK (verification IN ('unverified', 'verified', 'rejected')),
    review_note      text NOT NULL DEFAULT '',
    reviewed_by      uuid REFERENCES users (id) ON DELETE SET NULL,
    reviewed_at      timestamptz,
    submitted_at     timestamptz NOT NULL DEFAULT now(),
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_app_directory_listings_org
    ON app_directory_listings (organization_id);

CREATE INDEX IF NOT EXISTS idx_app_directory_listings_queue
    ON app_directory_listings (verification, submitted_at);
