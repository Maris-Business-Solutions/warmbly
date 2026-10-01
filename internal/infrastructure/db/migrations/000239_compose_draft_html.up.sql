-- A compose draft keeps the HTML body a template or the HTML editor gave it.
ALTER TABLE public.compose_drafts ADD COLUMN IF NOT EXISTS body_html text NOT NULL DEFAULT '';
