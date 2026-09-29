DROP TRIGGER IF EXISTS campaign_lead_enrol_cc_hold ON campaign_leads;
DROP FUNCTION IF EXISTS campaign_lead_enrol_cc_hold();
DROP TRIGGER IF EXISTS campaign_lead_cc_hold ON campaign_lead_cc;
DROP FUNCTION IF EXISTS campaign_lead_cc_hold();

DROP TABLE IF EXISTS campaign_lead_cc;

-- The source is gone, so its holds go with it rather than failing the check.
UPDATE campaign_leads
SET paused_at = NULL, paused_until = NULL, pause_reason = NULL, pause_source = NULL
WHERE pause_source = 'cc';

ALTER TABLE public.campaign_leads DROP CONSTRAINT IF EXISTS campaign_leads_pause_source_check;
ALTER TABLE public.campaign_leads
    ADD CONSTRAINT campaign_leads_pause_source_check
    CHECK (pause_source IS NULL OR pause_source IN ('manual', 'out_of_office', 'inbox_tagging')) NOT VALID;
