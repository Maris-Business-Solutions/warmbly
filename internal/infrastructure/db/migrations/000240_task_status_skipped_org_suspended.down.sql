-- Postgres cannot drop a single enum value, so 'skipped_org_suspended' stays on
-- task_status. Tasks parked in it return to the generic cancelled status.
UPDATE tasks SET status = 'cancelled' WHERE status = 'skipped_org_suspended';
