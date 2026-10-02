-- Where each workspace's follow-up sweep stopped, so a pass resumes its cycle; no row means start at the newest.
CREATE TABLE IF NOT EXISTS public.inbox_follow_up_sweeps (
    organization_id  uuid PRIMARY KEY REFERENCES public.organizations(id) ON DELETE CASCADE,
    -- No foreign key: a mailbox deleted mid-cycle still orders the walk.
    email_account_id uuid NOT NULL,
    internal_date    timestamptz NOT NULL,
    message_row_id   uuid NOT NULL,
    updated_at       timestamptz NOT NULL DEFAULT NOW()
);
