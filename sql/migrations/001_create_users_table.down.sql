DROP TRIGGER IF EXISTS update_users_updated_at ON users;
-- NOTE: update_updated_at_column() is a shared function used by multiple tables.
-- It should NOT be dropped here as other tables depend on it.
-- The function will be dropped automatically when no longer in use (no dependent triggers).
DROP INDEX IF EXISTS idx_users_username;
DROP INDEX IF EXISTS idx_users_email;
DROP TABLE IF EXISTS users;
