DROP TABLE IF EXISTS warmup_pending_filings;
DROP TRIGGER IF EXISTS warmup_thread_recovery ON warmup_thread_messages;
DROP TRIGGER IF EXISTS warmup_receipt_recovery ON warmup_received;
DROP TRIGGER IF EXISTS warmup_token_recovery ON warmup_tokens;
DROP FUNCTION IF EXISTS remember_warmup_thread_identifier();
DROP FUNCTION IF EXISTS remember_warmup_receipt_identifier();
DROP FUNCTION IF EXISTS remember_warmup_token_identifiers();
DROP FUNCTION IF EXISTS remember_warmup_identifier(uuid, text, timestamptz);
DROP TABLE IF EXISTS warmup_recovery_identifiers;
