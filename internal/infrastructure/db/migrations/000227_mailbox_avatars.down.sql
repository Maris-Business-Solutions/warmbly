ALTER TABLE public.email_accounts
    DROP COLUMN IF EXISTS avatar_checked_at,
    DROP COLUMN IF EXISTS avatar_url;
