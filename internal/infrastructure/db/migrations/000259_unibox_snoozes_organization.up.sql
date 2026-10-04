-- A snooze belongs to the organization whose inbox it hides a thread in.
ALTER TABLE public.unibox_snoozes
    ADD COLUMN organization_id uuid REFERENCES public.organizations (id) ON DELETE CASCADE;

ALTER TABLE public.unibox_snoozes DROP CONSTRAINT unibox_snoozes_user_id_thread_id_key;

DELETE FROM public.unibox_snoozes WHERE snoozed_until <= now();

-- One row per organization the thread's mail sits in, read the way the inbox
-- matches a snooze (the mail's user and thread).
INSERT INTO public.unibox_snoozes (user_id, thread_id, snoozed_until, created_at, updated_at, organization_id)
SELECT DISTINCT s.user_id, s.thread_id, s.snoozed_until, s.created_at, s.updated_at, ea.organization_id
FROM public.unibox_snoozes s
JOIN public.unibox_emails ue ON ue.user_id = s.user_id AND ue.thread_id = s.thread_id
JOIN public.email_accounts ea ON ea.id = ue.email_id
WHERE s.organization_id IS NULL AND ea.organization_id IS NOT NULL;

DELETE FROM public.unibox_snoozes WHERE organization_id IS NULL;

ALTER TABLE public.unibox_snoozes ALTER COLUMN organization_id SET NOT NULL;

ALTER TABLE public.unibox_snoozes
    ADD CONSTRAINT unibox_snoozes_org_user_thread_key UNIQUE (organization_id, user_id, thread_id);
