-- A test past the monthly free allowance is paid in credits. Each paid test
-- is settled once it finishes: refunded when it delivered nothing, kept
-- otherwise, with what came back recorded.
ALTER TABLE placement_tests
    ADD COLUMN credits_charged integer NOT NULL DEFAULT 0 CHECK (credits_charged >= 0),
    ADD COLUMN credits_refunded integer NOT NULL DEFAULT 0 CHECK (credits_refunded >= 0),
    ADD COLUMN credits_settled_at timestamptz,
    ADD COLUMN pace text NOT NULL DEFAULT 'spaced' CHECK (pace IN ('spaced', 'quick'));

-- The settle pass reads only paid tests it has not decided yet.
CREATE INDEX idx_placement_tests_credits_unsettled ON placement_tests (finished_at)
    WHERE credits_charged > 0 AND credits_settled_at IS NULL;
