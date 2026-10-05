ALTER TABLE fleet_nodes
    DROP COLUMN cpu_scope,
    DROP COLUMN memory_scope,
    DROP COLUMN memory_used_mb,
    DROP COLUMN memory_limit_mb,
    DROP COLUMN resident_mb;
