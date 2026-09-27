-- Spam placement only slows a mailbox down now, so a quarantine or block whose
-- recorded reason is placement is released: throttled with a spent term, and
-- re-judged on the next evaluation, which reapplies any complaint, bounce or
-- tampering band the metrics still show.
UPDATE public.warmup_pool_participants
   SET health_state = 'throttled',
       blocked_until = NOW()
 WHERE health_state IN ('quarantined', 'blocked')
   AND blocked_until IS NOT NULL
   AND blocked_until > NOW()
   AND COALESCE(last_health_reason, blocked_reason, '') ~ '^(catastrophic )?warmup spam placement ';

-- The same for a standing held against a removed address; a live row's mirror
-- was rewritten by its trigger above.
UPDATE public.warmup_reputation_ledger
   SET health_state = 'throttled',
       blocked_until = NOW(),
       standing_until = NOW()
 WHERE health_state IN ('quarantined', 'blocked')
   AND blocked_until IS NOT NULL
   AND blocked_until > NOW()
   AND COALESCE(last_health_reason, blocked_reason, '') ~ '^(catastrophic )?warmup spam placement ';
