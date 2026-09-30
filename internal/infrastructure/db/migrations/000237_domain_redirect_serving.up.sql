-- Where a redirect is served from, and whether visitors actually reach it.
--
-- served_by: 'instance' is this deployment's tracking service; 'cloud' is
-- Warmbly Cloud serving it for a linked self-hosted instance, whose verdict
-- and records this row mirrors (remote_records, remote_host).
--
-- linked_instance_id is the Cloud side of the same arrangement: the row a
-- linked instance asked Cloud to serve. It belongs to the link, not to the
-- workspace, so revoking the link removes it.
--
-- reach_* is the last HTTP check of the domain itself. DNS verification
-- proves the name points here; this proves a visitor gets the redirect.

ALTER TABLE public.domain_redirects
    ADD COLUMN served_by text NOT NULL DEFAULT 'instance'
        CONSTRAINT domain_redirects_served_by_check CHECK (served_by IN ('instance', 'cloud')),
    ADD COLUMN remote_host text NOT NULL DEFAULT '',
    ADD COLUMN remote_records jsonb,
    ADD COLUMN linked_instance_id uuid REFERENCES public.pool_link_instances (id) ON DELETE CASCADE,
    ADD COLUMN reach_status text
        CONSTRAINT domain_redirects_reach_status_check CHECK (reach_status IN ('ok', 'not_reaching', 'https_error', 'unreachable')),
    ADD COLUMN reach_hint text NOT NULL DEFAULT ''
        CONSTRAINT domain_redirects_reach_hint_check CHECK (reach_hint IN ('', 'not_routed', 'host_header', 'wrong_target', 'certificate', 'no_listener', 'settling')),
    ADD COLUMN reach_detail text NOT NULL DEFAULT '',
    ADD COLUMN reach_proxy text NOT NULL DEFAULT ''
        CONSTRAINT domain_redirects_reach_proxy_check CHECK (reach_proxy IN ('', 'traefik', 'nginx', 'caddy', 'apache', 'cloudflare', 'iis', 'litespeed')),
    ADD COLUMN reach_checked_at timestamptz;

CREATE INDEX idx_domain_redirects_linked_instance
    ON public.domain_redirects (linked_instance_id)
    WHERE linked_instance_id IS NOT NULL;
