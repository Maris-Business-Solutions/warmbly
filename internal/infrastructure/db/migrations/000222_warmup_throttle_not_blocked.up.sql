-- A throttle keeps a mailbox in the pool at reduced volume, so only a
-- quarantine or block carries blocked_at, which every partner query reads as
-- "out of the pool".
UPDATE public.warmup_pool_participants
   SET blocked_at = NULL
 WHERE blocked_at IS NOT NULL
   AND health_state NOT IN ('quarantined', 'blocked');
