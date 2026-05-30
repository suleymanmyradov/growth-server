-- Revert: drop concrete tables (data migrated back is not automatic; manual recovery needed)
DROP TABLE IF EXISTS saved_articles CASCADE;
DROP TABLE IF EXISTS saved_goals CASCADE;
DROP TABLE IF EXISTS saved_habits CASCADE;

-- Remove deprecation comment
COMMENT ON TABLE saved_items IS NULL;
