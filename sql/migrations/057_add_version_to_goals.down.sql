-- Revert optimistic locking on goals
DROP INDEX IF EXISTS idx_goals_version;
ALTER TABLE goals DROP COLUMN IF EXISTS version;
