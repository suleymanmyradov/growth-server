-- Add optimistic locking version column to habits
ALTER TABLE habits ADD COLUMN version INT NOT NULL DEFAULT 1;

-- Add index for version-based lookups (helps with UPDATE ... WHERE version = $n)
CREATE INDEX IF NOT EXISTS idx_habits_version ON habits(id, version);
