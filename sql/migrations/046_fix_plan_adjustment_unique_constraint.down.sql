-- Drop the partial unique indexes
DROP INDEX IF EXISTS uniq_plan_adjustments_habit;
DROP INDEX IF EXISTS uniq_plan_adjustments_goal;

-- Restore the original constraint
ALTER TABLE plan_adjustment_suggestions
ADD CONSTRAINT plan_adjustment_suggestions_unique_key
UNIQUE (user_id, source, week_start, habit_id, adjustment_type);
