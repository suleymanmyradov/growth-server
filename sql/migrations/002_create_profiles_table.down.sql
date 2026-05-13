DROP TRIGGER IF EXISTS update_profiles_updated_at ON profiles;
DROP INDEX IF EXISTS idx_profiles_user_id;
DROP TABLE IF EXISTS profiles;
