-- A placement batch runs the same test from many sending mailboxes. It is an
-- orchestration layer over placement_tests: each sender still gets its own
-- test, started by the backend a few at a time so a batch of thousands never
-- sends as a burst.
CREATE TABLE placement_batches (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    created_by      uuid REFERENCES users (id) ON DELETE SET NULL,
    campaign_id     uuid REFERENCES campaigns (id) ON DELETE SET NULL,
    sequence_id     uuid REFERENCES sequences (id) ON DELETE SET NULL,
    contact_id      uuid REFERENCES contacts (id) ON DELETE SET NULL,
    -- The copy, snapshotted at creation so every sender tests the same email.
    subject         text NOT NULL DEFAULT '',
    body_plain      text NOT NULL DEFAULT '',
    body_html       text NOT NULL DEFAULT '',
    tracking        text NOT NULL CHECK (tracking IN ('campaign', 'on', 'off', 'compare')),
    panel           text NOT NULL CHECK (panel IN ('instance', 'workspace', 'cloud')),
    pace            text NOT NULL CHECK (pace IN ('spaced', 'quick')),
    families        text[] NOT NULL DEFAULT '{}',
    seed_ids        uuid[] NOT NULL DEFAULT '{}',
    on_unavailable  text NOT NULL DEFAULT 'defer' CHECK (on_unavailable IN ('skip', 'defer')),
    -- How the senders were chosen, for display; the snapshot is the sender rows.
    selection       jsonb NOT NULL DEFAULT '{}',
    sender_count    integer NOT NULL CHECK (sender_count >= 0),
    max_credits     integer NOT NULL DEFAULT 0 CHECK (max_credits >= 0),
    credits_spent   integer NOT NULL DEFAULT 0 CHECK (credits_spent >= 0),
    status          text NOT NULL CHECK (status IN ('queued', 'running', 'completed', 'completed_with_warnings', 'cancelled', 'failed')),
    error           text NOT NULL DEFAULT '',
    -- Only an active batch is advanced by the runner. An imported batch lands
    -- inactive, so it never starts sending on the instance it moved to.
    active          boolean NOT NULL DEFAULT false,
    lease_until     timestamptz,
    last_tick_at    timestamptz,
    -- A deferred sender is retried until this moment, then skipped.
    retry_until     timestamptz NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT NOW(),
    started_at      timestamptz,
    finished_at     timestamptz,
    updated_at      timestamptz NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_placement_batches_org ON placement_batches (organization_id, created_at DESC);
CREATE INDEX idx_placement_batches_active ON placement_batches (lease_until) WHERE active;

-- One row per sender, written when the batch is created: the snapshot a later
-- tag or campaign edit cannot change.
CREATE TABLE placement_batch_senders (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id         uuid NOT NULL REFERENCES placement_batches (id) ON DELETE CASCADE,
    email_account_id uuid REFERENCES email_accounts (id) ON DELETE SET NULL,
    sender_email     text NOT NULL,
    sender_domain    text NOT NULL DEFAULT '',
    -- Who hosts the sending mailbox, a mailhost value.
    sender_family    text NOT NULL DEFAULT '',
    position         integer NOT NULL,
    status           text NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'deferred', 'running', 'completed', 'skipped', 'failed', 'cancelled')),
    -- A refusal identifier (placement_daily_budget, ...) and its sentence.
    reason           text NOT NULL DEFAULT '',
    detail           text NOT NULL DEFAULT '',
    attempts         integer NOT NULL DEFAULT 0,
    next_attempt_at  timestamptz NOT NULL DEFAULT NOW(),
    started_at       timestamptz,
    finished_at      timestamptz
);

CREATE UNIQUE INDEX idx_placement_batch_senders_account ON placement_batch_senders (batch_id, email_account_id);
CREATE INDEX idx_placement_batch_senders_due ON placement_batch_senders (batch_id, position)
    WHERE status IN ('queued', 'deferred');
CREATE INDEX idx_placement_batch_senders_running ON placement_batch_senders (batch_id) WHERE status = 'running';

ALTER TABLE placement_tests
    ADD COLUMN batch_id        uuid REFERENCES placement_batches (id) ON DELETE SET NULL,
    ADD COLUMN batch_sender_id uuid REFERENCES placement_batch_senders (id) ON DELETE SET NULL,
    DROP CONSTRAINT placement_tests_origin_check,
    ADD CONSTRAINT placement_tests_origin_check
        CHECK (origin IN ('manual', 'monitor', 'admin', 'remote', 'batch'));

CREATE INDEX idx_placement_tests_batch ON placement_tests (batch_id) WHERE batch_id IS NOT NULL;
CREATE INDEX idx_placement_tests_batch_sender ON placement_tests (batch_sender_id) WHERE batch_sender_id IS NOT NULL;
-- Fleet coverage reads each sender's latest test.
CREATE INDEX idx_placement_tests_sender_created ON placement_tests (sender_account_id, created_at DESC)
    WHERE sender_account_id IS NOT NULL;
