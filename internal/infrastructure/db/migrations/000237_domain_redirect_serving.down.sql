DELETE FROM public.domain_redirects WHERE linked_instance_id IS NOT NULL;

-- Without served_by every row reads as served here, so nothing Cloud verified may stay verified.
UPDATE public.domain_redirects SET verified = false, verified_at = NULL WHERE served_by = 'cloud';

DROP INDEX IF EXISTS public.idx_domain_redirects_linked_instance;

ALTER TABLE public.domain_redirects
    DROP COLUMN IF EXISTS reach_checked_at,
    DROP COLUMN IF EXISTS reach_proxy,
    DROP COLUMN IF EXISTS reach_detail,
    DROP COLUMN IF EXISTS reach_hint,
    DROP COLUMN IF EXISTS reach_status,
    DROP COLUMN IF EXISTS linked_instance_id,
    DROP COLUMN IF EXISTS remote_records,
    DROP COLUMN IF EXISTS remote_host,
    DROP COLUMN IF EXISTS served_by;
