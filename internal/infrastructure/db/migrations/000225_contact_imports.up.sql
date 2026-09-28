-- A contact import worked off in the background: uploaded once as a draft,
-- analysed, then run in chunks so a large file survives a closed tab, a proxy
-- timeout and a restarted backend.
CREATE TABLE public.contact_imports (
    id uuid PRIMARY KEY,
    organization_id uuid NOT NULL REFERENCES public.organizations (id) ON DELETE CASCADE,
    created_by uuid REFERENCES public.users (id) ON DELETE SET NULL,
    filename text NOT NULL DEFAULT '',
    format text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'queued', 'running', 'completed', 'failed', 'cancelled')),
    has_header boolean NOT NULL DEFAULT true,
    columns jsonb NOT NULL DEFAULT '[]'::jsonb,
    -- The mapping, duplicate handling and targets: autosaved while a draft,
    -- then what the import was started with.
    options jsonb NOT NULL DEFAULT '{}'::jsonb,
    -- What the mapper shows for a draft, so a reload resumes it; dropped once started.
    preview jsonb,
    total integer NOT NULL DEFAULT 0,
    quality jsonb,
    segments_pinned boolean,
    -- Notes about the whole import rather than one row.
    notes jsonb NOT NULL DEFAULT '[]'::jsonb,
    error text NOT NULL DEFAULT '',
    attempts integer NOT NULL DEFAULT 0,
    lease_until timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    started_at timestamptz,
    finished_at timestamptz
);

CREATE INDEX idx_contact_imports_org ON public.contact_imports (organization_id, created_at DESC, id DESC);
CREATE INDEX idx_contact_imports_due ON public.contact_imports (lease_until NULLS FIRST)
    WHERE status IN ('queued', 'running');

-- One row of the uploaded file, as uploaded, and what became of it.
CREATE TABLE public.contact_import_rows (
    import_id uuid NOT NULL REFERENCES public.contact_imports (id) ON DELETE CASCADE,
    line integer NOT NULL,
    cells jsonb NOT NULL DEFAULT '[]'::jsonb,
    status text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'imported', 'updated', 'skipped', 'failed')),
    email text NOT NULL DEFAULT '',
    reason text NOT NULL DEFAULT '',
    contact_id uuid REFERENCES public.contacts (id) ON DELETE SET NULL,
    PRIMARY KEY (import_id, line)
);

CREATE INDEX idx_contact_import_rows_status ON public.contact_import_rows (import_id, status);
CREATE INDEX idx_contact_import_rows_contact ON public.contact_import_rows (contact_id)
    WHERE contact_id IS NOT NULL;

-- A confirmed column mapping, keyed by the file's header set, so the next file
-- with the same headers maps itself.
CREATE TABLE public.contact_import_mappings (
    organization_id uuid NOT NULL REFERENCES public.organizations (id) ON DELETE CASCADE,
    signature text NOT NULL,
    mapping jsonb NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (organization_id, signature)
);
