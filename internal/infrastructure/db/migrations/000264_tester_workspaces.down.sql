UPDATE subscriptions SET managed_at = NULL, managed_by = NULL, managed_reason = NULL,
    managed_until = NULL, managed_plan_id = NULL, updated_at = now()
WHERE managed_plan_id = '00000000-0000-0000-0000-0000000000e1';

DELETE FROM plans p WHERE p.id = '00000000-0000-0000-0000-0000000000e1'
  AND NOT EXISTS (SELECT 1 FROM subscriptions s WHERE s.plan_id = p.id);

ALTER TABLE organizations DROP COLUMN category;
