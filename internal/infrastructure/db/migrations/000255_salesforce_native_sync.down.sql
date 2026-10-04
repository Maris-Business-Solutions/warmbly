DROP TABLE IF EXISTS salesforce_sync_state;
DROP TABLE IF EXISTS salesforce_import_members;
DROP TABLE IF EXISTS salesforce_import_sources;
DROP TABLE IF EXISTS salesforce_activity_queue;
DROP TABLE IF EXISTS salesforce_record_links;

UPDATE public.contacts SET source = 'import' WHERE source = 'crm_sync';
ALTER TABLE public.contacts DROP CONSTRAINT contacts_source_check;
ALTER TABLE public.contacts
    ADD CONSTRAINT contacts_source_check
    CHECK (source IN ('unknown', 'manual', 'campaign', 'import', 'sheet_sync', 'api', 'ai_assistant', 'form', 'automation')) NOT VALID;

ALTER TABLE integration_oauth_states DROP COLUMN IF EXISTS params;
