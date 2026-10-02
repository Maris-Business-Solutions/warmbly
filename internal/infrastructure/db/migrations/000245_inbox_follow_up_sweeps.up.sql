-- Each workspace's follow-up sweep bookkeeping: cycle cursor, changed-thread watermark, failure counts and walker lease.
CREATE TABLE IF NOT EXISTS public.inbox_follow_up_sweeps (
    organization_id  uuid PRIMARY KEY REFERENCES public.organizations(id) ON DELETE CASCADE,
    -- No foreign key: a mailbox deleted mid-cycle still orders the walk. NULL starts a cycle at the newest.
    email_account_id uuid,
    internal_date    timestamptz,
    message_row_id   uuid,
    -- Changes up to (fresh_at, fresh_row_id) have been checked.
    fresh_at         timestamptz,
    fresh_row_id     uuid,
    page_failures    integer NOT NULL DEFAULT 0,
    fresh_failures   integer NOT NULL DEFAULT 0,
    lease_owner      uuid,
    leased_until     timestamptz,
    updated_at       timestamptz NOT NULL DEFAULT NOW()
);
