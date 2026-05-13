DROP TRIGGER IF EXISTS update_habits_updated_at ON habits;
DROP INDEX IF EXISTS idx_habits_not_completed;
DROP INDEX IF EXISTS idx_habits_category;
DROP INDEX IF EXISTS idx_habits_user_id;
DROP TABLE IF EXISTS habits;
