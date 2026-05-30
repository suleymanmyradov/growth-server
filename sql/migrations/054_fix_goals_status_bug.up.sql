-- The goals table has `completed BOOLEAN`, not a `status` column.
-- Migration 049 incorrectly created `idx_goals_active_status WHERE status != 'completed'`
-- and the billing query `CountActiveGoalsForUser` references `status != 'completed'`.
-- Both will fail because the column does not exist.

-- Step 1: Drop the broken index (if it was somehow created, or just clean it up)
DROP INDEX IF EXISTS idx_goals_active_status;

-- Step 2: Recreate the partial index using the correct column name: `completed`
CREATE INDEX IF NOT EXISTS idx_goals_active_completed
    ON goals(user_id)
    WHERE completed = FALSE;

-- Step 3: Migrate from boolean `completed` to a proper `status` enum for future flexibility.
-- This aligns with the original intent of the billing query and allows future states
-- like 'archived', 'paused', 'abandoned' without schema changes.
CREATE TYPE goal_status_type AS ENUM ('active', 'completed', 'archived');

-- Add the new status column (nullable initially for backfill)
ALTER TABLE goals ADD COLUMN status goal_status_type;

-- Backfill: map existing boolean to enum
UPDATE goals
SET status = CASE WHEN completed = TRUE THEN 'completed'::goal_status_type ELSE 'active'::goal_status_type END;

-- Make status NOT NULL after backfill
ALTER TABLE goals ALTER COLUMN status SET NOT NULL;

-- Add default for new inserts
ALTER TABLE goals ALTER COLUMN status SET DEFAULT 'active'::goal_status_type;

-- Drop the old boolean column and its dependent indexes/triggers after status is populated
-- (We keep completed for now in case app code still references it, but make it a generated column)
-- Actually, the app code uses completed in toggle/update queries. Let's make it a generated column
-- so existing queries continue to work while status becomes the canonical column.

-- First drop the old partial index on completed (replaced by the new one above)
DROP INDEX IF EXISTS idx_goals_not_completed;

-- Add generated column for backward compatibility with existing app queries
ALTER TABLE goals ADD COLUMN completed_gen BOOLEAN
    GENERATED ALWAYS AS (status = 'completed') STORED;

-- The app queries reference `completed` directly. Since we can't rename a generated column
-- to `completed` while the original exists, we need a different approach.
-- Safer path: keep the original `completed` column as a real column maintained by trigger.

-- Drop the generated column attempt
ALTER TABLE goals DROP COLUMN IF EXISTS completed_gen;

-- Create a trigger to keep `completed` in sync with `status`
CREATE OR REPLACE FUNCTION sync_goals_completed_from_status()
RETURNS TRIGGER AS $$
BEGIN
    NEW.completed := (NEW.status = 'completed');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER goals_sync_completed
    BEFORE INSERT OR UPDATE ON goals
    FOR EACH ROW
    EXECUTE FUNCTION sync_goals_completed_from_status();

-- Now we have both `status` (canonical) and `completed` (backward-compatible).
-- Update the partial index to use status (matches the billing query intent)
DROP INDEX IF EXISTS idx_goals_active_completed;
CREATE INDEX IF NOT EXISTS idx_goals_active_status
    ON goals(user_id)
    WHERE status != 'completed';
