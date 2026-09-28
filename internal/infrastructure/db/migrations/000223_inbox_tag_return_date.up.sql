-- The out-of-office return date a verdict was asked to confirm, so the hold
-- reads the answer only for the date it parsed itself. Nullable: no rewrite.
ALTER TABLE inbox_tag_results
    ADD COLUMN IF NOT EXISTS return_date date;
