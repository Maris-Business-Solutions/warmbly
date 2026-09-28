-- A mailbox's own profile photo, read from its provider or inbox vendor where one is set.
ALTER TABLE public.email_accounts
    ADD COLUMN IF NOT EXISTS avatar_url text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS avatar_checked_at timestamptz;
