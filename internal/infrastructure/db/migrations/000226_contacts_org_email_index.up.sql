-- Alone in its file for CONCURRENTLY. An import looks every address of a file
-- up in the workspace at once, and the unique index is per member.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_contacts_org_email
    ON public.contacts (organization_id, lower(email));
