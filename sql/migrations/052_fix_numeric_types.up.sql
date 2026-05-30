-- Change weekly_reviews.completion_rate from NUMERIC to proper type and ensure NOT NULL
-- Already NUMERIC(5,2) NOT NULL in schema, but Go side needs a float mapping.
-- This migration is a no-op at DB level but ensures sqlc picks up the latest state.
-- (No actual ALTER needed since migration 024 already defined it correctly.)

-- Fix any potential invalid values in plan limits that might have been inserted
-- after migration 050 but before this one.
UPDATE plans SET active_goal_limit = 0 WHERE active_goal_limit IS NULL;
UPDATE plans SET active_habit_limit = 0 WHERE active_habit_limit IS NULL;
UPDATE plans SET weekly_review_history_limit = 0 WHERE weekly_review_history_limit IS NULL;
UPDATE plans SET plan_adjustment_limit = 0 WHERE plan_adjustment_limit IS NULL;
UPDATE categories SET sort_order = 0 WHERE sort_order IS NULL;
