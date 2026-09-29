-- When a deletion strike was confirmed by searching the mailbox for the
-- message. Rows recorded before that search existed stay NULL and are searched
-- once by the consumer's recheck; every row written from now on is confirmed.
ALTER TABLE warmup_tampering_events ADD COLUMN IF NOT EXISTS verified_at timestamptz;
ALTER TABLE warmup_tampering_events ALTER COLUMN verified_at SET DEFAULT now();
