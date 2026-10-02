-- When the row was stored (an imported row keeps its source value); created_at is the worker's sync clock. Existing rows read as epoch.
ALTER TABLE public.unibox_emails ADD COLUMN IF NOT EXISTS ingested_at timestamptz NOT NULL DEFAULT 'epoch';
ALTER TABLE public.unibox_emails ALTER COLUMN ingested_at SET DEFAULT NOW();
