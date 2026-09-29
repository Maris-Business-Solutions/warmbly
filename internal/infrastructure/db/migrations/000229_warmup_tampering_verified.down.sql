DROP INDEX IF EXISTS idx_warmup_tampering_unverified;
ALTER TABLE warmup_tampering_events DROP COLUMN IF EXISTS verify_requested_at;
ALTER TABLE warmup_tampering_events DROP COLUMN IF EXISTS verified_at;
