-- Alone in its file for CONCURRENTLY. The health read walks one sender's
-- receipts over seven days; the primary key starts at the recipient.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_warmup_received_sender
    ON warmup_received (sender_account_id, created_at);
