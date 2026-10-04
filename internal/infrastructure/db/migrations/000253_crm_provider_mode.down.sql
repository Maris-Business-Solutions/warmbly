UPDATE public.campaign_leads SET paused_at = NULL, paused_until = NULL, pause_reason = NULL, pause_source = NULL
WHERE pause_source = 'crm';
ALTER TABLE public.campaign_leads DROP CONSTRAINT IF EXISTS campaign_leads_pause_source_check;
ALTER TABLE public.campaign_leads
    ADD CONSTRAINT campaign_leads_pause_source_check
    CHECK (pause_source IS NULL OR pause_source IN ('manual', 'out_of_office', 'inbox_tagging', 'cc')) NOT VALID;

DROP TABLE IF EXISTS crm_sync_cursors;
DROP TABLE IF EXISTS crm_sync_jobs;
DROP TABLE IF EXISTS crm_owners;
DROP TABLE IF EXISTS crm_contact_records;
DROP TABLE IF EXISTS crm_external_links;
DROP TABLE IF EXISTS crm_settings;
