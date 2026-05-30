-- Revert goals.status additions and restore boolean-only state

-- Drop the sync trigger and function
DROP TRIGGER IF EXISTS goals_sync_completed ON goals;
DROP FUNCTION IF EXISTS sync_goals_completed_from_status();

-- Drop the status-based index, recreate the original boolean one
DROP INDEX IF EXISTS idx_goals_active_status;
CREATE INDEX IF NOT EXISTS idx_goals_not_completed ON goals(user_id) WHERE completed = FALSE;

-- Drop the status column (app must not depend on it in down-migration scenarios)
ALTER TABLE goals DROP COLUMN IF EXISTS status;

-- Drop the enum type
DROP TYPE IF EXISTS goal_status_type;
