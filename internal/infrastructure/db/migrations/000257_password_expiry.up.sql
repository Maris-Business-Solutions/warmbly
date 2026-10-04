-- A password an operator hands out (a tester account) stops working at this
-- time. NULL means the password does not expire.
ALTER TABLE public.users
    ADD COLUMN password_expires_at timestamptz;
