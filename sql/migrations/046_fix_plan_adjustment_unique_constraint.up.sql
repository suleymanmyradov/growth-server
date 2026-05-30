-- The existing unique constraint (user_id, source, week_start, habit_id, adjustment_type)
-- is broken for goal-based adjustments because PostgreSQL does not consider NULL = NULL.
-- When habit_id is NULL (goal-based), duplicate rows can be inserted.
-- Fix: add a generated column that unifies goal_id/habit_id, then recreate the constraint
-- so that the sqlc ON CONFLICT query never references a missing constraint.

-- Step 1: Drop the broken constraint (this also drops its backing index)
ALTER TABLE plan_adjustment_suggestions
DROP CONSTRAINT IF EXISTS plan_adjustment_suggestions_unique_key;

-- Step 2: Add generated column representing the single non-null target
ALTER TABLE plan_adjustment_suggestions
ADD COLUMN target_id UUID GENERATED ALWAYS AS (COALESCE(goal_id, habit_id)) STORED;

-- Step 3: Create unified unique constraint using the generated column.
-- This preserves the original constraint name so the sqlc query keeps working.
ALTER TABLE plan_adjustment_suggestions
ADD CONSTRAINT plan_adjustment_suggestions_unique_key
UNIQUE (user_id, source, week_start, target_id, adjustment_type);
