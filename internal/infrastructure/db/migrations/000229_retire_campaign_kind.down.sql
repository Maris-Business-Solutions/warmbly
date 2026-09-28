ALTER TABLE campaigns ADD COLUMN IF NOT EXISTS kind text NOT NULL DEFAULT 'sequence'
    CHECK (kind IN ('sequence', 'one_time'));
