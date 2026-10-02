-- When this instance stored the row; created_at is the worker's sync clock. Existing rows read as epoch, never as new.
ALTER TABLE public.unibox_emails ADD COLUMN IF NOT EXISTS ingested_at timestamptz NOT NULL DEFAULT 'epoch';
ALTER TABLE public.unibox_emails ALTER COLUMN ingested_at SET DEFAULT NOW();
