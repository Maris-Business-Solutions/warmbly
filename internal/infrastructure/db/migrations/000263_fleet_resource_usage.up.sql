ALTER TABLE fleet_nodes
    ADD COLUMN cpu_scope text NOT NULL DEFAULT '',
    ADD COLUMN memory_scope text NOT NULL DEFAULT '',
    ADD COLUMN memory_used_mb integer,
    ADD COLUMN memory_limit_mb integer,
    ADD COLUMN resident_mb integer;
