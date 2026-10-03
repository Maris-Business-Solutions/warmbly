-- Alone in its file for CONCURRENTLY. FindRoutedPairs enumerates a campaign's
-- candidate leads in routing order before hydrating them in bounded chunks;
-- the manual ("position") ordering becomes an index scan with this, instead of
-- sorting the whole campaign's membership every scheduling pass.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_campaign_leads_campaign_position
    ON public.campaign_leads USING btree (campaign_id, "position", contact_id);
