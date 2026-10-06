ALTER TABLE organizations ADD COLUMN category text NOT NULL DEFAULT 'standard'
    CHECK (category IN ('standard', 'test'));

INSERT INTO plans (id, name, max_contacts, daily_emails, ai_generation, account_limit,
    price, discounted_price, duration_id, savings, public, monthly_credits,
    max_campaigns, max_active_campaigns, max_team_members, max_email_accounts, daily_campaign_limit)
VALUES ('00000000-0000-0000-0000-0000000000e1', 'Test', 10000, 1000, true, 10,
    0, 0, (SELECT id FROM durations WHERE title = 'month'), 0, false, 100,
    100, 20, 10, 10, 100);

UPDATE users u SET password_expires_at = a.created_at + interval '30 days', updated_at = now()
FROM admin_audit_logs a
WHERE a.action = 'create_tester' AND a.target_id = u.id
  AND u.login_code_exempt AND u.password_expires_at IS NULL;

UPDATE users SET onboarding_completed_at = COALESCE(login_code_exempt_at, created_at), updated_at = now()
WHERE onboarding_completed_at IS NULL AND login_code_exempt AND password_expires_at IS NOT NULL;

UPDATE organizations o SET category = 'test', updated_at = now()
FROM users u
WHERE o.owner_user_id = u.id
  AND EXISTS (
      SELECT 1 FROM admin_audit_logs a
      WHERE a.action = 'create_tester' AND a.target_id = u.id
        AND a.details->>'organization_id' = o.id::text
        AND COALESCE(a.details->>'joined_existing', 'false') = 'false'
  );

INSERT INTO subscriptions (user_id, organization_id, plan_id, stripe_customer_id,
    managed_at, managed_by, managed_reason, managed_until, managed_plan_id)
SELECT u.id, o.id, '00000000-0000-0000-0000-000000000001', '',
    now(), u.login_code_exempt_by, u.login_code_exempt_reason, u.password_expires_at,
    '00000000-0000-0000-0000-0000000000e1'
FROM organizations o JOIN users u ON u.id = o.owner_user_id
WHERE o.category = 'test' AND u.login_code_exempt AND u.password_expires_at > now()
  AND NOT EXISTS (SELECT 1 FROM subscriptions s WHERE s.organization_id = o.id);

UPDATE subscriptions s SET managed_at = now(), managed_by = u.login_code_exempt_by,
    managed_reason = u.login_code_exempt_reason, managed_until = u.password_expires_at,
    managed_plan_id = '00000000-0000-0000-0000-0000000000e1', updated_at = now()
FROM organizations o JOIN users u ON u.id = o.owner_user_id
WHERE s.organization_id = o.id AND o.category = 'test' AND u.login_code_exempt AND u.password_expires_at > now()
  AND s.managed_at IS NULL AND s.stripe_subscription_id IS NULL;

WITH granted AS (
    INSERT INTO credit_ledger (org_id, balance)
    SELECT o.id, p.monthly_credits
    FROM organizations o JOIN subscriptions s ON s.organization_id = o.id
    JOIN plans p ON p.id = s.managed_plan_id
    WHERE o.category = 'test' AND p.id = '00000000-0000-0000-0000-0000000000e1'
      AND s.managed_until > now()
      AND NOT EXISTS (SELECT 1 FROM credit_ledger_transactions t WHERE t.idempotency_key = o.id::text || ':tester_grant')
    ON CONFLICT (org_id) DO UPDATE SET balance = credit_ledger.balance + EXCLUDED.balance, updated_at = now()
    RETURNING org_id, balance, purchased_balance
)
INSERT INTO credit_ledger_transactions (org_id, amount, reason, balance_after, purchased_balance_after, idempotency_key)
SELECT org_id, p.monthly_credits, 'tester_grant', balance, purchased_balance, org_id::text || ':tester_grant'
FROM granted CROSS JOIN plans p WHERE p.id = '00000000-0000-0000-0000-0000000000e1';
