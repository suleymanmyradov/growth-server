-- Fix plan_adjustment_suggestions ON CONFLICT after migration 046.
-- Migration 046 replaced the single unique constraint with two partial unique indexes
-- because PostgreSQL unique indexes treat NULL != NULL. This broke the sqlc query
-- `ON CONFLICT (user_id, source, week_start, habit_id, adjustment_type)`.
--
-- Solution: add a `target_id` generated column that stores whichever of goal_id/habit_id
-- is non-NULL (the XOR constraint guarantees exactly one is set). Then create a single
-- non-partial unique constraint on (user_id, source, week_start, target_id, adjustment_type)
-- that the INSERT ... ON CONFLICT query can target directly.

-- Step 1: add generated column representing the single non-null target
ALTER TABLE plan_adjustment_suggestions
ADD COLUMN target_id UUID GENERATED ALWAYS AS (COALESCE(goal_id, habit_id)) STORED;

-- Step 2: add the unified unique constraint
ALTER TABLE plan_adjustment_suggestions
ADD CONSTRAINT plan_adjustment_suggestions_unique_key
UNIQUE (user_id, source, week_start, target_id, adjustment_type);

-- Step 3: drop the now-redundant partial unique indexes from migration 046
DROP INDEX IF EXISTS uniq_plan_adjustments_habit;
DROP INDEX IF EXISTS uniq_plan_adjustments_goal;
