CREATE TABLE warmup_recovery_identifiers (
    organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    mailbox_hash bytea NOT NULL,
    identifier_hash bytea NOT NULL,
    expires_at timestamptz NOT NULL,
    PRIMARY KEY (organization_id, mailbox_hash, identifier_hash)
);
CREATE INDEX warmup_recovery_identifiers_expiry ON warmup_recovery_identifiers (expires_at);

CREATE FUNCTION remember_warmup_identifier(account_id uuid, identifier text, recorded_at timestamptz)
RETURNS void LANGUAGE sql AS $$
    INSERT INTO warmup_recovery_identifiers (organization_id, mailbox_hash, identifier_hash, expires_at)
    SELECT organization_id, sha256(convert_to(lower(btrim(email)), 'UTF8')),
           sha256(convert_to(identifier, 'UTF8')), recorded_at + INTERVAL '90 days'
    FROM email_accounts
    WHERE id = account_id AND organization_id IS NOT NULL
      AND identifier <> '' AND recorded_at > NOW() - INTERVAL '90 days'
    ON CONFLICT (organization_id, mailbox_hash, identifier_hash)
    DO UPDATE SET expires_at = GREATEST(warmup_recovery_identifiers.expires_at, EXCLUDED.expires_at);
$$;

CREATE FUNCTION remember_warmup_token_identifiers() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    PERFORM remember_warmup_identifier(NEW.sender_account_id, 'token:' || NEW.token, NEW.created_at);
    PERFORM remember_warmup_identifier(NEW.recipient_account_id, 'token:' || NEW.token, NEW.created_at);
    IF btrim(NEW.sent_message_id, '<> ') <> '' THEN
        PERFORM remember_warmup_identifier(NEW.sender_account_id, 'message:' || btrim(NEW.sent_message_id, '<> '), NEW.created_at);
        PERFORM remember_warmup_identifier(NEW.recipient_account_id, 'message:' || btrim(NEW.sent_message_id, '<> '), NEW.created_at);
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER warmup_token_recovery AFTER INSERT OR UPDATE OF sent_message_id ON warmup_tokens
    FOR EACH ROW EXECUTE FUNCTION remember_warmup_token_identifiers();

CREATE FUNCTION remember_warmup_receipt_identifier() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF btrim(NEW.message_id, '<> ') <> '' THEN
        PERFORM remember_warmup_identifier(NEW.email_account_id, 'message:' || btrim(NEW.message_id, '<> '), NEW.created_at);
        PERFORM remember_warmup_identifier(NEW.sender_account_id, 'message:' || btrim(NEW.message_id, '<> '), NEW.created_at);
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER warmup_receipt_recovery AFTER INSERT OR UPDATE OF message_id ON warmup_received
    FOR EACH ROW EXECUTE FUNCTION remember_warmup_receipt_identifier();

CREATE FUNCTION remember_warmup_thread_identifier() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF btrim(NEW.message_id, '<> ') <> '' THEN
        PERFORM remember_warmup_identifier(NEW.email_account_id, 'message:' || btrim(NEW.message_id, '<> '), NEW.created_at);
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER warmup_thread_recovery AFTER INSERT ON warmup_thread_messages
    FOR EACH ROW EXECUTE FUNCTION remember_warmup_thread_identifier();

SELECT remember_warmup_identifier(account_id, identifier, created_at)
FROM (
    SELECT sender_account_id AS account_id, 'token:' || token AS identifier, created_at FROM warmup_tokens
    UNION ALL SELECT recipient_account_id, 'token:' || token, created_at FROM warmup_tokens
    UNION ALL SELECT sender_account_id, 'message:' || btrim(sent_message_id, '<> '), created_at FROM warmup_tokens WHERE btrim(sent_message_id, '<> ') <> ''
    UNION ALL SELECT recipient_account_id, 'message:' || btrim(sent_message_id, '<> '), created_at FROM warmup_tokens WHERE btrim(sent_message_id, '<> ') <> ''
    UNION ALL SELECT email_account_id, 'message:' || btrim(message_id, '<> '), created_at FROM warmup_received WHERE btrim(message_id, '<> ') <> ''
    UNION ALL SELECT sender_account_id, 'message:' || btrim(message_id, '<> '), created_at FROM warmup_received WHERE btrim(message_id, '<> ') <> ''
    UNION ALL SELECT email_account_id, 'message:' || btrim(message_id, '<> '), created_at FROM warmup_thread_messages WHERE btrim(message_id, '<> ') <> ''
) identifiers WHERE created_at > NOW() - INTERVAL '90 days';

CREATE TABLE warmup_pending_filings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email_account_id uuid NOT NULL REFERENCES email_accounts(id) ON DELETE CASCADE,
    message_key text NOT NULL,
    payload jsonb NOT NULL,
    next_attempt_at timestamptz NOT NULL DEFAULT NOW(),
    created_at timestamptz NOT NULL DEFAULT NOW(),
    UNIQUE (email_account_id, message_key)
);
CREATE INDEX warmup_pending_filings_due ON warmup_pending_filings (next_attempt_at);
