DROP INDEX IF EXISTS idx_crm_sync_jobs_due_provider;

UPDATE crm_settings SET provider = 'native' WHERE provider = 'pipedrive';
DELETE FROM crm_external_links WHERE provider = 'pipedrive';
DELETE FROM crm_contact_records WHERE provider = 'pipedrive';
DELETE FROM crm_owners WHERE provider = 'pipedrive';
DELETE FROM crm_sync_jobs WHERE provider = 'pipedrive';
DELETE FROM crm_sync_cursors WHERE provider = 'pipedrive';

DROP INDEX IF EXISTS idx_crm_sync_jobs_dedupe;
CREATE UNIQUE INDEX idx_crm_sync_jobs_dedupe ON crm_sync_jobs (organization_id, dedupe_key)
    WHERE dedupe_key IS NOT NULL AND status IN ('pending', 'running');

ALTER TABLE crm_settings DROP CONSTRAINT IF EXISTS crm_settings_provider_check;
ALTER TABLE crm_settings ADD CONSTRAINT crm_settings_provider_check CHECK (provider IN ('native', 'hubspot'));
ALTER TABLE crm_external_links DROP CONSTRAINT IF EXISTS crm_external_links_provider_check;
ALTER TABLE crm_external_links ADD CONSTRAINT crm_external_links_provider_check CHECK (provider IN ('hubspot'));
ALTER TABLE crm_contact_records DROP CONSTRAINT IF EXISTS crm_contact_records_provider_check;
ALTER TABLE crm_contact_records ADD CONSTRAINT crm_contact_records_provider_check CHECK (provider IN ('hubspot'));
ALTER TABLE crm_owners DROP CONSTRAINT IF EXISTS crm_owners_provider_check;
ALTER TABLE crm_owners ADD CONSTRAINT crm_owners_provider_check CHECK (provider IN ('hubspot'));
ALTER TABLE crm_sync_jobs DROP CONSTRAINT IF EXISTS crm_sync_jobs_provider_check;
ALTER TABLE crm_sync_jobs ADD CONSTRAINT crm_sync_jobs_provider_check CHECK (provider IN ('hubspot'));
ALTER TABLE crm_sync_cursors DROP CONSTRAINT IF EXISTS crm_sync_cursors_provider_check;
ALTER TABLE crm_sync_cursors ADD CONSTRAINT crm_sync_cursors_provider_check CHECK (provider IN ('hubspot'));
