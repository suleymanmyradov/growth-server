-- Remove composite indexes for plan_adjustment_suggestions table
DROP INDEX IF EXISTS idx_plan_adjustment_suggestions_user_status_created;
DROP INDEX IF EXISTS idx_plan_adjustment_suggestions_goal_id;
