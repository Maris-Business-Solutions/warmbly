-- verified_at: when a search of the mailbox confirmed the strike; NULL on rows from before the search.
-- verify_requested_at: when the consumer last asked a worker to search for one of those.
ALTER TABLE warmup_tampering_events ADD COLUMN IF NOT EXISTS verified_at timestamptz;
ALTER TABLE warmup_tampering_events ALTER COLUMN verified_at SET DEFAULT now();
ALTER TABLE warmup_tampering_events ADD COLUMN IF NOT EXISTS verify_requested_at timestamptz;
CREATE INDEX IF NOT EXISTS idx_warmup_tampering_unverified
    ON warmup_tampering_events (created_at) WHERE verified_at IS NULL;
