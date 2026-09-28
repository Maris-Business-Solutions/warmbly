DROP INDEX IF EXISTS idx_placement_tests_credits_unsettled;
ALTER TABLE placement_tests
    DROP COLUMN IF EXISTS pace,
    DROP COLUMN IF EXISTS credits_settled_at,
    DROP COLUMN IF EXISTS credits_refunded,
    DROP COLUMN IF EXISTS credits_charged;
