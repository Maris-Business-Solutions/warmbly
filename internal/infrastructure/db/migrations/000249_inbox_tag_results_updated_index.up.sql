-- Alone in its file for CONCURRENTLY. The follow-up sweep reads a workspace's newly written verdicts in order.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_inbox_tag_results_updated
    ON public.inbox_tag_results USING btree (organization_id, updated_at, id);
