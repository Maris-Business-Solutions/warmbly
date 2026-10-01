-- Alone in its file for CONCURRENTLY. A mailbox's status reads its newest
-- warmup send the server refused.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_tasks_account_warmup_failed
    ON tasks (email_account_id, updated_at)
    WHERE status = 'failed' AND task_type = 'warmup';
