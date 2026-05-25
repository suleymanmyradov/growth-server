-- Add composite indexes for plan_adjustment_suggestions table
CREATE INDEX idx_plan_adjustment_suggestions_user_status_created
ON plan_adjustment_suggestions(user_id, status, created_at DESC);

CREATE INDEX idx_plan_adjustment_suggestions_goal_id
ON plan_adjustment_suggestions(goal_id);
