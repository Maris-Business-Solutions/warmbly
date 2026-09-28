ALTER TABLE placement_tests
    DROP COLUMN IF EXISTS pace,
    DROP COLUMN IF EXISTS credits_refunded_at,
    DROP COLUMN IF EXISTS credits_charged;
