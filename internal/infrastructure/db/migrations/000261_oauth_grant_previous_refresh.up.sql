-- The refresh token a grant last rotated away from, so presenting it again ends the grant.
ALTER TABLE oauth_access_grants
    ADD COLUMN IF NOT EXISTS previous_refresh_token_hash text;

CREATE INDEX IF NOT EXISTS idx_oauth_access_grants_previous_refresh
    ON oauth_access_grants (previous_refresh_token_hash)
    WHERE previous_refresh_token_hash IS NOT NULL;
