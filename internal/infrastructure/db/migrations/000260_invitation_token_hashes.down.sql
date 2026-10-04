-- Digests cannot be reversed; pending invitations need to be sent again after a downgrade.
DROP INDEX IF EXISTS organization_invitations_link_token_hash_key;
ALTER TABLE organization_invitations DROP COLUMN IF EXISTS link_token_hash;
