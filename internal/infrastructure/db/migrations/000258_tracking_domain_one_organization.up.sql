-- A custom tracking host is verified by at most one organization on an
-- instance, across mailbox and campaign tracking domains.

-- Where several organizations hold one already, the earliest verification
-- keeps it and the others go back to unverified.
WITH holders AS (
    SELECT tracking_domain AS host, organization_id, min(tracking_domain_verified_at) AS first_at
    FROM (
        SELECT tracking_domain, organization_id, tracking_domain_verified_at
        FROM public.email_accounts
        WHERE tracking_domain_verified AND tracking_domain <> ''
        UNION ALL
        SELECT tracking_domain, organization_id, tracking_domain_verified_at
        FROM public.campaigns
        WHERE tracking_domain_verified AND tracking_domain <> ''
    ) verified
    GROUP BY 1, 2
), keeper AS (
    SELECT DISTINCT ON (host) host, organization_id
    FROM holders
    ORDER BY host, first_at ASC NULLS LAST, organization_id
), mailboxes AS (
    UPDATE public.email_accounts ea
    SET tracking_domain_verified = false, tracking_domain_verified_at = NULL
    FROM keeper k
    WHERE ea.tracking_domain = k.host
      AND ea.tracking_domain_verified
      AND ea.organization_id IS DISTINCT FROM k.organization_id
    RETURNING ea.id
)
UPDATE public.campaigns c
SET tracking_domain_verified = false, tracking_domain_verified_at = NULL
FROM keeper k
WHERE c.tracking_domain = k.host
  AND c.tracking_domain_verified
  AND c.organization_id IS DISTINCT FROM k.organization_id;

CREATE FUNCTION public.tracking_domain_one_organization() RETURNS trigger
    LANGUAGE plpgsql AS $$
BEGIN
    IF NOT NEW.tracking_domain_verified OR NEW.tracking_domain = '' THEN
        RETURN NEW;
    END IF;
    -- Serializes concurrent verifications of one host.
    PERFORM pg_advisory_xact_lock(hashtextextended('tracking_domain:' || NEW.tracking_domain, 0));
    IF EXISTS (
        SELECT 1 FROM public.email_accounts
        WHERE tracking_domain = NEW.tracking_domain AND tracking_domain_verified
          AND organization_id IS DISTINCT FROM NEW.organization_id
        UNION ALL
        SELECT 1 FROM public.campaigns
        WHERE tracking_domain = NEW.tracking_domain AND tracking_domain_verified
          AND organization_id IS DISTINCT FROM NEW.organization_id
    ) THEN
        RAISE EXCEPTION 'tracking domain is verified by another organization'
            USING ERRCODE = 'unique_violation', CONSTRAINT = 'tracking_domain_one_organization';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER tracking_domain_one_organization
    BEFORE INSERT OR UPDATE OF tracking_domain, tracking_domain_verified, organization_id ON public.email_accounts
    FOR EACH ROW WHEN (NEW.tracking_domain_verified)
    EXECUTE FUNCTION public.tracking_domain_one_organization();

CREATE TRIGGER tracking_domain_one_organization
    BEFORE INSERT OR UPDATE OF tracking_domain, tracking_domain_verified, organization_id ON public.campaigns
    FOR EACH ROW WHEN (NEW.tracking_domain_verified)
    EXECUTE FUNCTION public.tracking_domain_one_organization();
