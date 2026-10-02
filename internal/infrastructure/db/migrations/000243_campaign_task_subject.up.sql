-- The subject a campaign send carried, rendered: a follow-up threads on what the
-- contact was actually sent, A/B arm included. NULL on older rows, which fall
-- back to walking the steps.
ALTER TABLE campaign_tasks
    ADD COLUMN IF NOT EXISTS subject text;
