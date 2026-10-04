-- A workspace runs one CRM: Warmbly's own, or a connected provider (HubSpot).
-- In provider mode the local CRM tables are a mirror of the provider's records,
-- written through on every change, so every dashboard surface, search and
-- realtime path keeps reading Postgres.
CREATE TABLE crm_settings (
    organization_id    uuid PRIMARY KEY REFERENCES organizations (id) ON DELETE CASCADE,
    provider           text NOT NULL DEFAULT 'native' CHECK (provider IN ('native', 'hubspot')),
    connection_id      uuid REFERENCES integration_connections (id) ON DELETE SET NULL,
    -- models.CRMProviderConfig, validated on write.
    config             jsonb NOT NULL DEFAULT '{}'::jsonb,
    setup_completed_at timestamptz,
    created_at         timestamptz NOT NULL DEFAULT NOW(),
    updated_at         timestamptz NOT NULL DEFAULT NOW()
);

-- Which local CRM row mirrors which provider record.
CREATE TABLE crm_external_links (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    provider        text NOT NULL CHECK (provider IN ('hubspot')),
    object_type     text NOT NULL CHECK (object_type IN ('pipeline', 'stage', 'deal', 'task', 'note', 'meeting', 'email')),
    local_id        uuid NOT NULL,
    external_id     text NOT NULL,
    -- Provider facts with no local column (stage probability, the deal owner
    -- when the owner is not a member), read back for display.
    meta            jsonb NOT NULL DEFAULT '{}'::jsonb,
    synced_at       timestamptz NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, provider, object_type, external_id),
    UNIQUE (organization_id, provider, object_type, local_id)
);

CREATE INDEX idx_crm_external_links_local ON crm_external_links (local_id);

-- The provider's view of a Warmbly contact: its record id and the sales fields
-- Warmbly has no column for.
CREATE TABLE crm_contact_records (
    organization_id     uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    contact_id          uuid NOT NULL REFERENCES contacts (id) ON DELETE CASCADE,
    provider            text NOT NULL CHECK (provider IN ('hubspot')),
    external_id         text NOT NULL,
    owner_external_id   text,
    lifecycle_stage     text,
    lead_status         text,
    company_external_id text,
    company_name        text,
    company_domain      text,
    opted_out           boolean NOT NULL DEFAULT false,
    -- Extra provider properties the workspace chose to display, read whole.
    properties          jsonb NOT NULL DEFAULT '{}'::jsonb,
    external_updated_at timestamptz,
    synced_at           timestamptz NOT NULL DEFAULT NOW(),
    PRIMARY KEY (contact_id, provider),
    UNIQUE (organization_id, provider, external_id)
);

CREATE INDEX idx_crm_contact_records_org ON crm_contact_records (organization_id, provider);

-- The provider's users who can own records, matched to workspace members.
CREATE TABLE crm_owners (
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    provider        text NOT NULL CHECK (provider IN ('hubspot')),
    external_id     text NOT NULL,
    email           text NOT NULL DEFAULT '',
    first_name      text NOT NULL DEFAULT '',
    last_name       text NOT NULL DEFAULT '',
    user_id         uuid REFERENCES users (id) ON DELETE SET NULL,
    -- True once a person chose the match, so an email re-match never overrides it.
    user_pinned     boolean NOT NULL DEFAULT false,
    archived        boolean NOT NULL DEFAULT false,
    synced_at       timestamptz NOT NULL DEFAULT NOW(),
    PRIMARY KEY (organization_id, provider, external_id)
);

CREATE INDEX idx_crm_owners_user ON crm_owners (user_id) WHERE user_id IS NOT NULL;

-- Outbox of changes headed for the provider. Drained by the consumer with
-- SKIP LOCKED, retried with backoff, and kept after failure so the workspace
-- can see and retry what did not sync.
CREATE TABLE crm_sync_jobs (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    provider        text NOT NULL CHECK (provider IN ('hubspot')),
    kind            text NOT NULL CHECK (kind IN (
        'log_email', 'log_event', 'contact_props', 'push_contact', 'push_deal', 'push_task',
        'push_note', 'log_meeting', 'reply_outcome', 'refresh_object', 'backfill'
    )),
    -- Collapses repeat work: a second pending job with the same key is dropped.
    dedupe_key      text,
    -- What the job is about, shown in the sync health list.
    subject         text NOT NULL DEFAULT '',
    payload         jsonb NOT NULL DEFAULT '{}'::jsonb,
    status          text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'done', 'failed')),
    attempts        integer NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT NOW(),
    locked_until    timestamptz,
    last_error      text,
    created_at      timestamptz NOT NULL DEFAULT NOW(),
    updated_at      timestamptz NOT NULL DEFAULT NOW(),
    finished_at     timestamptz
);

CREATE INDEX idx_crm_sync_jobs_due ON crm_sync_jobs (next_attempt_at) WHERE status = 'pending';
CREATE INDEX idx_crm_sync_jobs_org ON crm_sync_jobs (organization_id, status, updated_at DESC);
CREATE UNIQUE INDEX idx_crm_sync_jobs_dedupe ON crm_sync_jobs (organization_id, dedupe_key)
    WHERE dedupe_key IS NOT NULL AND status IN ('pending', 'running');

-- Incremental pull checkpoints, one per object type. Instance-local.
CREATE TABLE crm_sync_cursors (
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    provider        text NOT NULL CHECK (provider IN ('hubspot')),
    object_type     text NOT NULL,
    cursor_at       timestamptz,
    last_run_at     timestamptz,
    last_error      text,
    PRIMARY KEY (organization_id, provider, object_type)
);

-- A provider rule (a deal opened, a lifecycle stage reached) holds a lead with
-- no end, like a person's decision.
ALTER TABLE public.campaign_leads DROP CONSTRAINT IF EXISTS campaign_leads_pause_source_check;
ALTER TABLE public.campaign_leads
    ADD CONSTRAINT campaign_leads_pause_source_check
    CHECK (pause_source IS NULL OR pause_source IN ('manual', 'out_of_office', 'inbox_tagging', 'cc', 'crm')) NOT VALID;
