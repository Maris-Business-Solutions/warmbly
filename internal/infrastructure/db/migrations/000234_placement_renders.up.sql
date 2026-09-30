-- The copy both halves of a tracking comparison send to one seed, rendered once
-- and sealed with the workspace's key, so tracking is the only difference.
CREATE TABLE IF NOT EXISTS placement_renders (
    compare_group_id uuid        NOT NULL,
    seed_address     text        NOT NULL,
    organization_id  uuid        NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    content          text        NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (compare_group_id, seed_address)
);
CREATE INDEX IF NOT EXISTS idx_placement_renders_org ON placement_renders (organization_id);
