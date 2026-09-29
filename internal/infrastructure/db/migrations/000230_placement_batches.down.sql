DROP INDEX IF EXISTS idx_placement_tests_sender_created;
DROP INDEX IF EXISTS idx_placement_tests_batch_sender;
DROP INDEX IF EXISTS idx_placement_tests_batch;
UPDATE placement_tests SET origin = 'manual' WHERE origin = 'batch';
ALTER TABLE placement_tests
    DROP CONSTRAINT placement_tests_origin_check,
    ADD CONSTRAINT placement_tests_origin_check
        CHECK (origin IN ('manual', 'monitor', 'admin', 'remote')),
    DROP COLUMN batch_sender_id,
    DROP COLUMN batch_id;
DROP TABLE IF EXISTS placement_batch_senders;
DROP TABLE IF EXISTS placement_batches;
