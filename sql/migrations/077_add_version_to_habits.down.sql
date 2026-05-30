DROP INDEX IF EXISTS idx_habits_version;
ALTER TABLE habits DROP COLUMN IF EXISTS version;
