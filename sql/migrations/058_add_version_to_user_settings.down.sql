-- Revert optimistic locking on user_settings
DROP INDEX IF EXISTS idx_user_settings_version;
ALTER TABLE user_settings DROP COLUMN IF EXISTS version;
