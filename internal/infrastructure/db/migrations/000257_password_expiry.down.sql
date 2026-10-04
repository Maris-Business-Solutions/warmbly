ALTER TABLE public.users
    DROP COLUMN IF EXISTS password_expires_at;
