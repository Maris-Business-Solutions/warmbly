ALTER TABLE public.unibox_snoozes DROP CONSTRAINT IF EXISTS unibox_snoozes_org_user_thread_key;

-- Keep the latest snooze per (user, thread) so the old key can return.
DELETE FROM public.unibox_snoozes s
USING public.unibox_snoozes newer
WHERE s.user_id = newer.user_id AND s.thread_id = newer.thread_id
  AND (s.snoozed_until, s.id) < (newer.snoozed_until, newer.id);

ALTER TABLE public.unibox_snoozes
    ADD CONSTRAINT unibox_snoozes_user_id_thread_id_key UNIQUE (user_id, thread_id);

ALTER TABLE public.unibox_snoozes DROP COLUMN organization_id;
