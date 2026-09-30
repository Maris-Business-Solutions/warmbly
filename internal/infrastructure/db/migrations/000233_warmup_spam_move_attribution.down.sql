DROP TABLE IF EXISTS mailbox_owner_activity;
DROP TABLE IF EXISTS warmup_spam_moves;
ALTER TABLE warmup_received DROP COLUMN IF EXISTS landed_spam;
