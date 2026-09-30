-- The refresh nonce a session rotated away from, so a replay of an older
-- refresh token can be told apart from two tabs refreshing at once.
ALTER TABLE sessions ADD COLUMN previous_refresh_nonce text;
