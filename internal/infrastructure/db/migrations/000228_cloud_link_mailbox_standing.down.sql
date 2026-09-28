ALTER TABLE cloud_link_mailboxes DROP CONSTRAINT IF EXISTS cloud_link_mailboxes_health_state_check;
ALTER TABLE cloud_link_mailboxes
    DROP COLUMN IF EXISTS health_synced_at,
    DROP COLUMN IF EXISTS health_evaluated_at,
    DROP COLUMN IF EXISTS blocked_until,
    DROP COLUMN IF EXISTS health_score,
    DROP COLUMN IF EXISTS health_pool_type,
    DROP COLUMN IF EXISTS health_reason,
    DROP COLUMN IF EXISTS health_state;
