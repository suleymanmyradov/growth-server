-- Add optimistic locking version column to user_settings
ALTER TABLE user_settings ADD COLUMN version INT NOT NULL DEFAULT 1;

-- Add index for version-based lookups
CREATE INDEX IF NOT EXISTS idx_user_settings_version ON user_settings(user_id, version);
