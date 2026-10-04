DROP INDEX IF EXISTS idx_oauth_access_grants_previous_refresh;
ALTER TABLE oauth_access_grants DROP COLUMN IF EXISTS previous_refresh_token_hash;
