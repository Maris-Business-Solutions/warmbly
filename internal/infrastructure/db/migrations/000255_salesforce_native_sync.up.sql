-- Native Salesforce sync: a Warmbly contact is linked to the Lead or Contact it
-- is in Salesforce, campaign activity is logged there as Tasks through a durable
-- outbox, and list views or Salesforce Campaigns feed contacts in.

-- Which Salesforce login host a handshake started against (production, sandbox
-- or a My Domain), so the code is exchanged where it was issued.
ALTER TABLE integration_oauth_states
    ADD COLUMN params jsonb NOT NULL DEFAULT '{}'::jsonb;

-- Contacts imported from a CRM list or campaign are a first-touch origin.
ALTER TABLE public.contacts DROP CONSTRAINT contacts_source_check;
ALTER TABLE public.contacts
    ADD CONSTRAINT contacts_source_check
    CHECK (source IN ('unknown', 'manual', 'campaign', 'import', 'sheet_sync', 'api', 'ai_assistant', 'form', 'automation', 'crm_sync')) NOT VALID;

-- One row per Warmbly contact per Salesforce connection: the record it is, and a
-- snapshot of what the dashboard shows about it. record_id is always 18 chars.
CREATE TABLE salesforce_record_links (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id    uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    connection_id      uuid NOT NULL REFERENCES integration_connections (id) ON DELETE CASCADE,
    contact_id         uuid NOT NULL REFERENCES contacts (id) ON DELETE CASCADE,
    sobject            text NOT NULL CHECK (sobject IN ('Lead', 'Contact')),
    record_id          varchar(18) NOT NULL,
    account_id         varchar(18),
    account_name       text NOT NULL DEFAULT '',
    owner_id           varchar(18),
    owner_name         text NOT NULL DEFAULT '',
    lead_status        text NOT NULL DEFAULT '',
    is_converted       boolean NOT NULL DEFAULT false,
    opted_out          boolean NOT NULL DEFAULT false,
    snapshot           jsonb NOT NULL DEFAULT '{}'::jsonb,
    linked_by          text NOT NULL DEFAULT 'match' CHECK (linked_by IN ('match', 'created', 'import', 'manual')),
    record_modified_at timestamptz,
    last_pushed_at     timestamptz,
    last_pulled_at     timestamptz,
    last_error         text,
    last_error_at      timestamptz,
    created_at         timestamptz NOT NULL DEFAULT NOW(),
    updated_at         timestamptz NOT NULL DEFAULT NOW(),
    UNIQUE (connection_id, contact_id)
);

CREATE INDEX idx_salesforce_record_links_record ON salesforce_record_links (connection_id, record_id);
CREATE INDEX idx_salesforce_record_links_contact ON salesforce_record_links (organization_id, contact_id);

-- The activity outbox: every campaign event to log in Salesforce, written when
-- the event happens and drained in batches. Doubles as the per-record sync log.
-- content_encrypted holds subject and body text sealed with the org DEK.
CREATE TABLE salesforce_activity_queue (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id   uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    connection_id     uuid NOT NULL REFERENCES integration_connections (id) ON DELETE CASCADE,
    contact_id        uuid REFERENCES contacts (id) ON DELETE SET NULL,
    contact_email     text NOT NULL,
    kind              text NOT NULL CHECK (kind IN ('sent', 'opened', 'clicked', 'replied', 'bounced', 'unsubscribed', 'meeting_booked')),
    dedupe_key        text NOT NULL,
    payload           jsonb NOT NULL DEFAULT '{}'::jsonb,
    content_encrypted text NOT NULL DEFAULT '',
    status            text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'synced', 'skipped', 'failed')),
    attempts          integer NOT NULL DEFAULT 0,
    -- The drain pass holding the row; its outcome only lands while it still does.
    lease_id          uuid,
    next_attempt_at   timestamptz NOT NULL DEFAULT NOW(),
    sf_record_id      varchar(18),
    sf_task_id        varchar(18),
    detail            text NOT NULL DEFAULT '',
    occurred_at       timestamptz NOT NULL,
    created_at        timestamptz NOT NULL DEFAULT NOW(),
    processed_at      timestamptz,
    UNIQUE (connection_id, dedupe_key)
);

CREATE INDEX idx_salesforce_activity_queue_due ON salesforce_activity_queue (next_attempt_at) WHERE status = 'pending';
CREATE INDEX idx_salesforce_activity_queue_conn ON salesforce_activity_queue (connection_id, created_at DESC);
CREATE INDEX idx_salesforce_activity_queue_contact ON salesforce_activity_queue (contact_id) WHERE contact_id IS NOT NULL;

-- A saved Salesforce list view or Campaign that feeds contacts in, once or on
-- a schedule, optionally straight into a Warmbly campaign.
CREATE TABLE salesforce_import_sources (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id    uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    connection_id      uuid NOT NULL REFERENCES integration_connections (id) ON DELETE CASCADE,
    created_by_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    name               text NOT NULL DEFAULT '',
    source_kind        text NOT NULL CHECK (source_kind IN ('list_view', 'campaign')),
    sobject            text NOT NULL CHECK (sobject IN ('Lead', 'Contact', 'CampaignMember')),
    source_id          varchar(18) NOT NULL,
    source_label       text NOT NULL DEFAULT '',
    campaign_id        uuid REFERENCES campaigns (id) ON DELETE SET NULL,
    category_ids       uuid[] NOT NULL DEFAULT '{}',
    recurring          boolean NOT NULL DEFAULT false,
    enabled            boolean NOT NULL DEFAULT true,
    status             text NOT NULL DEFAULT 'idle' CHECK (status IN ('idle', 'running', 'error')),
    last_run_at        timestamptz,
    last_result        jsonb,
    last_error         text NOT NULL DEFAULT '',
    total_imported     integer NOT NULL DEFAULT 0,
    created_at         timestamptz NOT NULL DEFAULT NOW(),
    updated_at         timestamptz NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_salesforce_import_sources_conn ON salesforce_import_sources (connection_id);

-- Which records a source has already brought in, so a recurring run only
-- imports people new to the view instead of rewriting every contact.
CREATE TABLE salesforce_import_members (
    source_id     uuid NOT NULL REFERENCES salesforce_import_sources (id) ON DELETE CASCADE,
    record_id     varchar(18) NOT NULL,
    contact_id    uuid REFERENCES contacts (id) ON DELETE SET NULL,
    first_seen_at timestamptz NOT NULL DEFAULT NOW(),
    PRIMARY KEY (source_id, record_id)
);

-- Where the pull loop got to in each connection, and the API budget it saw.
-- Instance-local: an imported connection starts from its own first pull.
CREATE TABLE salesforce_sync_state (
    connection_id   uuid PRIMARY KEY REFERENCES integration_connections (id) ON DELETE CASCADE,
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    lead_cursor     timestamptz,
    contact_cursor  timestamptz,
    opportunity_cursor timestamptz,
    last_pull_at    timestamptz,
    last_pull_error text NOT NULL DEFAULT '',
    api_used        integer NOT NULL DEFAULT 0,
    api_max         integer NOT NULL DEFAULT 0,
    api_seen_at     timestamptz,
    -- Warmbly's own calls on calls_day, held against the connection's budget.
    calls_day       date,
    calls_today     integer NOT NULL DEFAULT 0,
    updated_at      timestamptz NOT NULL DEFAULT NOW()
);
