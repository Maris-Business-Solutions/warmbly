-- Every campaign is a sequence: a first email and optional follow-ups. A
-- former one-time email keeps its single step and runs as any other campaign.
ALTER TABLE campaigns DROP COLUMN IF EXISTS kind;
