DROP INDEX IF EXISTS idx_placement_tests_refund_owed;
ALTER TABLE placement_tests
    DROP COLUMN IF EXISTS pace,
    DROP COLUMN IF EXISTS credits_refunded_at,
    DROP COLUMN IF EXISTS credits_charged;
