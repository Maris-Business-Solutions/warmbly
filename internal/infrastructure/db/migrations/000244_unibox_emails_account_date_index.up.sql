-- Alone in its file for CONCURRENTLY. The follow-up sweep pages one mailbox newest first on (internal_date, id).
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_unibox_emails_account_date
    ON public.unibox_emails USING btree (email_id, internal_date, id);
