-- The warmup standing Warmbly Cloud last reported for a mailbox it warms, so
-- this instance's send gates hold it to the same verdict as a local pool row.
ALTER TABLE cloud_link_mailboxes
    ADD COLUMN IF NOT EXISTS health_state        text,
    ADD COLUMN IF NOT EXISTS health_reason       text,
    ADD COLUMN IF NOT EXISTS health_pool_type    text,
    ADD COLUMN IF NOT EXISTS health_score        double precision NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS blocked_until       timestamptz,
    ADD COLUMN IF NOT EXISTS health_evaluated_at timestamptz,
    ADD COLUMN IF NOT EXISTS health_synced_at    timestamptz;

ALTER TABLE cloud_link_mailboxes
    ADD CONSTRAINT cloud_link_mailboxes_health_state_check
        CHECK (health_state IS NULL OR health_state IN ('healthy', 'watch', 'throttled', 'quarantined', 'blocked'));
