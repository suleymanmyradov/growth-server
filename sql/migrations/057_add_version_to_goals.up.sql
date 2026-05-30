-- Add optimistic locking version column to goals
ALTER TABLE goals ADD COLUMN version INT NOT NULL DEFAULT 1;

-- Add index for version-based lookups (helps with UPDATE ... WHERE version = $n)
CREATE INDEX IF NOT EXISTS idx_goals_version ON goals(id, version);
