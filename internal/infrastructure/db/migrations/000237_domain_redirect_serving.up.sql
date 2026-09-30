-- Where a redirect is served from (remote_* mirrors Cloud when it serves it), the link a Cloud row belongs to, and the last visit to the domain.

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

-- One workspace per instance hands a domain to Cloud, which keeps one row per domain for the link.
CREATE UNIQUE INDEX idx_domain_redirects_cloud_domain ON public.domain_redirects (domain) WHERE served_by = 'cloud';

CREATE INDEX idx_domain_redirects_linked_instance
    ON public.domain_redirects (linked_instance_id)
    WHERE linked_instance_id IS NOT NULL;
