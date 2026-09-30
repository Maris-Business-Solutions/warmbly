-- landed_spam: the warmup email was in the recipient's spam folder when it arrived, so a spam label on it is the filter's, not the owner's.
ALTER TABLE warmup_received ADD COLUMN IF NOT EXISTS landed_spam boolean NOT NULL DEFAULT false;

UPDATE warmup_received wr
SET landed_spam = true
WHERE wr.message_id <> ''
  AND EXISTS (
    SELECT 1 FROM warmup_spam_reports sr
    WHERE sr.reporter_account_id = wr.email_account_id
      AND sr.message_id = wr.message_id
      AND sr.report_type = 'spam_placement'
  );

-- A spam strike on mail that arrived in spam was never the owner's act.
DELETE FROM warmup_tampering_events t
USING warmup_received wr
WHERE t.kind = 'spam_flag'
  AND wr.email_account_id = t.email_account_id
  AND wr.message_id = t.message_id
  AND wr.landed_spam;

-- A received warmup email moved to spam after arrival, held until the activity around it attributes it.
CREATE TABLE IF NOT EXISTS warmup_spam_moves (
    email_account_id  uuid        NOT NULL REFERENCES email_accounts(id) ON DELETE CASCADE,
    message_id        text        NOT NULL,
    sender_account_id uuid        NOT NULL REFERENCES email_accounts(id) ON DELETE CASCADE,
    received_at       timestamptz NOT NULL,
    observed_at       timestamptz NOT NULL DEFAULT now(),
    verdict           text        NOT NULL DEFAULT 'pending'
        CHECK (verdict IN ('pending', 'owner', 'provider', 'unattributed')),
    signals           text[]      NOT NULL DEFAULT '{}',
    decided_at        timestamptz,
    PRIMARY KEY (email_account_id, message_id)
);
CREATE INDEX IF NOT EXISTS idx_warmup_spam_moves_pending ON warmup_spam_moves (observed_at) WHERE verdict = 'pending';
CREATE INDEX IF NOT EXISTS idx_warmup_spam_moves_sender ON warmup_spam_moves (sender_account_id, observed_at);

-- Five-minute buckets in which the owner acted on their own mail at the provider (read, unread, star), never Warmbly's echo.
CREATE TABLE IF NOT EXISTS mailbox_owner_activity (
    email_account_id uuid        NOT NULL REFERENCES email_accounts(id) ON DELETE CASCADE,
    bucket           timestamptz NOT NULL,
    events           integer     NOT NULL DEFAULT 1,
    PRIMARY KEY (email_account_id, bucket)
);
CREATE INDEX IF NOT EXISTS idx_mailbox_owner_activity_bucket ON mailbox_owner_activity (bucket);
