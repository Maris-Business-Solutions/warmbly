-- Invitation tokens are stored as SHA-256 hex digests; the plaintext exists only
-- in the email and the copied link. A copied link has its own token, so copying
-- one never invalidates the emailed link.
ALTER TABLE organization_invitations
    ADD COLUMN IF NOT EXISTS link_token_hash character varying(64);

CREATE UNIQUE INDEX IF NOT EXISTS organization_invitations_link_token_hash_key
    ON organization_invitations (link_token_hash)
    WHERE link_token_hash IS NOT NULL;

UPDATE organization_invitations
   SET token = encode(sha256(convert_to(token, 'UTF8')), 'hex');
