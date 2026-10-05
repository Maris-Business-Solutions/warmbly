-- Synthetic telemetry for the isolated rich-seed QA database, never production.
BEGIN;
DO $$
BEGIN
  IF left(current_database(), 11) <> 'warmbly_qa_' THEN
    RAISE EXCEPTION 'fleet proof fixtures require an isolated warmbly_qa_ database';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM users WHERE email = 'dev@warmbly.com') THEN
    RAISE EXCEPTION 'fleet proof fixtures require the rich QA seed';
  END IF;
END $$;
UPDATE fleet_nodes SET address='', last_seen_at=NULL, active=true,
  cpu_percent=NULL,cpu_scope='',memory_scope='',memory_used_mb=NULL,memory_limit_mb=NULL,resident_mb=NULL,
  capacity_target=100, name='fixture-never-' || right(id::text,3), notes='Synthetic localhost fleet proof fixture';
UPDATE workers SET account_count=0,load_score=0,health_state='healthy';
DELETE FROM worker_health_samples;
UPDATE fleet_nodes SET name='fixture-container',address='8.8.8.8',last_seen_at=now(),
  cpu_percent=12.5,cpu_scope='container',memory_scope='container',
  memory_used_mb=256,memory_limit_mb=1024,resident_mb=137,capacity_target=200
  WHERE id='10c8f5e4-1c39-5b2a-9c8b-3d2f0a8b1a01';
UPDATE fleet_nodes SET name='fixture-host',address='1.1.1.1',last_seen_at=now(),
  cpu_percent=7.5,cpu_scope='host',memory_scope='host',
  memory_used_mb=4096,memory_limit_mb=16384,resident_mb=80,capacity_target=400
  WHERE id='10c8f5e4-1c39-5b2a-9c8b-3d2f0a8b1a02';
UPDATE fleet_nodes SET name='fixture-unavailable',last_seen_at=now(),memory_mb=99
  WHERE id='10c8f5e4-1c39-5b2a-9c8b-3d2f0a8b1a03';
UPDATE fleet_nodes SET name='fixture-stale',last_seen_at=now()-interval '20 minutes',
  cpu_percent=88,cpu_scope='host',memory_scope='host',memory_used_mb=800,
  memory_limit_mb=1024,resident_mb=999
  WHERE id='00000000-0000-0000-0000-000000000201';
INSERT INTO worker_health_samples(worker_id,sends_attempted,sends_succeeded)
  VALUES ('10c8f5e4-1c39-5b2a-9c8b-3d2f0a8b1a01',80,72),
         ('10c8f5e4-1c39-5b2a-9c8b-3d2f0a8b1a02',20,20);
-- Keep the view's planning inputs obsolete to exercise current row values.
REFRESH MATERIALIZED VIEW worker_capacity_view;
UPDATE fleet_nodes SET capacity_target=400 WHERE name='fixture-container';
UPDATE workers SET account_count=400 WHERE id='10c8f5e4-1c39-5b2a-9c8b-3d2f0a8b1a01';
UPDATE workers SET account_count=100 WHERE id='10c8f5e4-1c39-5b2a-9c8b-3d2f0a8b1a02';
COMMIT;
