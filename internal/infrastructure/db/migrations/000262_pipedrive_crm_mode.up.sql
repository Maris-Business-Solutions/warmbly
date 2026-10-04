-- Pipedrive joins HubSpot as a CRM a workspace can run on. It shares the mirror,
-- the outbox and the pull checkpoints, each row keyed by its provider.

-- Drop the provider CHECKs by what they say rather than by an assumed name.
DO $$
DECLARE
    c record;
BEGIN
    FOR c IN
        SELECT con.conname, rel.relname
        FROM pg_constraint con
        JOIN pg_class rel ON rel.oid = con.conrelid
        JOIN pg_namespace ns ON ns.oid = rel.relnamespace
        WHERE con.contype = 'c'
          AND ns.nspname = current_schema()
          AND rel.relname IN ('crm_settings', 'crm_external_links', 'crm_contact_records', 'crm_owners', 'crm_sync_jobs', 'crm_sync_cursors')
          AND pg_get_constraintdef(con.oid) LIKE '%provider%'
          AND pg_get_constraintdef(con.oid) LIKE '%hubspot%'
    LOOP
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT %I', c.relname, c.conname);
    END LOOP;
END $$;

ALTER TABLE crm_settings
    ADD CONSTRAINT crm_settings_provider_check CHECK (provider IN ('native', 'hubspot', 'pipedrive'));
ALTER TABLE crm_external_links
    ADD CONSTRAINT crm_external_links_provider_check CHECK (provider IN ('hubspot', 'pipedrive'));
ALTER TABLE crm_contact_records
    ADD CONSTRAINT crm_contact_records_provider_check CHECK (provider IN ('hubspot', 'pipedrive'));
ALTER TABLE crm_owners
    ADD CONSTRAINT crm_owners_provider_check CHECK (provider IN ('hubspot', 'pipedrive'));
ALTER TABLE crm_sync_jobs
    ADD CONSTRAINT crm_sync_jobs_provider_check CHECK (provider IN ('hubspot', 'pipedrive'));
ALTER TABLE crm_sync_cursors
    ADD CONSTRAINT crm_sync_cursors_provider_check CHECK (provider IN ('hubspot', 'pipedrive'));

-- One provider's queued work never swallows another's with the same key.
DROP INDEX IF EXISTS idx_crm_sync_jobs_dedupe;
CREATE UNIQUE INDEX idx_crm_sync_jobs_dedupe ON crm_sync_jobs (organization_id, provider, dedupe_key)
    WHERE dedupe_key IS NOT NULL AND status IN ('pending', 'running');

-- Each drainer claims only its own provider's jobs.
CREATE INDEX IF NOT EXISTS idx_crm_sync_jobs_due_provider
    ON crm_sync_jobs (provider, next_attempt_at) WHERE status = 'pending';
