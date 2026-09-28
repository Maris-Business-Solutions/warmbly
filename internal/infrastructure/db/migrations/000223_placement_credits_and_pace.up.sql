-- A test past the monthly free allowance is paid in credits; the charge is
-- recorded on the test so a test that delivered nothing is refunded once.
ALTER TABLE placement_tests
    ADD COLUMN credits_charged integer NOT NULL DEFAULT 0 CHECK (credits_charged >= 0),
    ADD COLUMN credits_refunded_at timestamptz,
    ADD COLUMN pace text NOT NULL DEFAULT 'spaced' CHECK (pace IN ('spaced', 'quick'));

-- The refund pass reads only paid tests still owed a decision.
CREATE INDEX idx_placement_tests_refund_owed ON placement_tests (finished_at)
    WHERE credits_charged > 0 AND credits_refunded_at IS NULL;
