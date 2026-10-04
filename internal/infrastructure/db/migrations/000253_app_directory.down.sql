DROP TABLE IF EXISTS oauth_developer_blocks;
ALTER TABLE oauth_applications
    DROP COLUMN IF EXISTS suspended_by,
    DROP COLUMN IF EXISTS suspended_reason,
    DROP COLUMN IF EXISTS suspended_at;
DROP TABLE IF EXISTS app_directory_listings;
