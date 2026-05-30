-- The existing unique constraint (user_id, source, week_start, habit_id, adjustment_type)
-- is broken for goal-based adjustments because PostgreSQL does not consider NULL = NULL.
-- When habit_id is NULL (goal-based), duplicate rows can be inserted.
-- Fix: replace with two partial unique indexes that cover both target types.

-- Drop the broken constraint (this also drops its backing index)
ALTER TABLE plan_adjustment_suggestions
DROP CONSTRAINT IF EXISTS plan_adjustment_suggestions_unique_key;

-- Partial unique index for habit-based adjustments
CREATE UNIQUE INDEX uniq_plan_adjustments_habit
ON plan_adjustment_suggestions(user_id, source, week_start, habit_id, adjustment_type)
WHERE habit_id IS NOT NULL;

-- Partial unique index for goal-based adjustments
CREATE UNIQUE INDEX uniq_plan_adjustments_goal
ON plan_adjustment_suggestions(user_id, source, week_start, goal_id, adjustment_type)
WHERE goal_id IS NOT NULL;
