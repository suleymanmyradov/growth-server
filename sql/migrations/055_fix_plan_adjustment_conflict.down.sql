-- Revert plan_adjustment_suggestions conflict fix

-- Drop the unified unique constraint
ALTER TABLE plan_adjustment_suggestions
DROP CONSTRAINT IF EXISTS plan_adjustment_suggestions_unique_key;

-- Drop the generated column
ALTER TABLE plan_adjustment_suggestions DROP COLUMN IF EXISTS target_id;

-- Recreate the partial unique indexes from migration 046
CREATE UNIQUE INDEX uniq_plan_adjustments_habit
ON plan_adjustment_suggestions(user_id, source, week_start, habit_id, adjustment_type)
WHERE habit_id IS NOT NULL;

CREATE UNIQUE INDEX uniq_plan_adjustments_goal
ON plan_adjustment_suggestions(user_id, source, week_start, goal_id, adjustment_type)
WHERE goal_id IS NOT NULL;
