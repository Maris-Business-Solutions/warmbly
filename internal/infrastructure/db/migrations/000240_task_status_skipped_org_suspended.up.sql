-- The warmup task records a suspended workspace's skip under this status.
-- Postgres cannot drop an enum value, so the down migration leaves it.
ALTER TYPE public.task_status ADD VALUE IF NOT EXISTS 'skipped_org_suspended';
