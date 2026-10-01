-- A mailbox adopted into a linked instance warms on the cloud. Start warmup on
-- the adopted mailboxes that never started it, unless a member stopped it since
-- or it is a placement seed, which never warms.
UPDATE email_accounts ea
SET warmup = now(), warmup_paused_at = NULL
FROM pool_link_mailboxes plm
WHERE plm.email_account_id = ea.id
  AND plm.managed
  AND ea.status = 'active'
  AND ea.warmup IS NULL
  AND ea.seed_scope IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM audit_logs a
      WHERE a.organization_id = ea.organization_id
        AND a.entity_id = ea.id
        AND a.created_at >= plm.enrolled_at
        AND a.changes ? 'warmup'
  );
