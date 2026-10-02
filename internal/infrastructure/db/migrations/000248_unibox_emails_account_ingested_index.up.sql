-- Alone in its file for CONCURRENTLY. The follow-up sweep reads each mailbox's newly stored messages in order.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_unibox_emails_account_ingested
    ON public.unibox_emails USING btree (email_id, ingested_at, id);
