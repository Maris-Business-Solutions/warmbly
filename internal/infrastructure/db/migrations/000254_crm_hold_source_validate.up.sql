-- Validates the pause_source CHECK 000253 re-added NOT VALID, in its own
-- transaction so writes continue while it scans.
ALTER TABLE public.campaign_leads VALIDATE CONSTRAINT campaign_leads_pause_source_check;
