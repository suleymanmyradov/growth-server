-- Set safe defaults for existing NULL rows before adding NOT NULL constraints
UPDATE plans SET active_goal_limit = 0 WHERE active_goal_limit IS NULL;
UPDATE plans SET active_habit_limit = 0 WHERE active_habit_limit IS NULL;
UPDATE plans SET weekly_review_history_limit = 0 WHERE weekly_review_history_limit IS NULL;
UPDATE plans SET plan_adjustment_limit = 0 WHERE plan_adjustment_limit IS NULL;
UPDATE categories SET sort_order = 0 WHERE sort_order IS NULL;

-- habits: streak and completed have DEFAULTs, make NOT NULL
ALTER TABLE habits ALTER COLUMN streak SET NOT NULL;
ALTER TABLE habits ALTER COLUMN completed SET NOT NULL;

-- goals: progress and completed have DEFAULTs, make NOT NULL
ALTER TABLE goals ALTER COLUMN progress SET NOT NULL;
ALTER TABLE goals ALTER COLUMN completed SET NOT NULL;

-- plans: limit columns now have defaults, make NOT NULL
ALTER TABLE plans ALTER COLUMN active_goal_limit SET NOT NULL;
ALTER TABLE plans ALTER COLUMN active_habit_limit SET NOT NULL;
ALTER TABLE plans ALTER COLUMN weekly_review_history_limit SET NOT NULL;
ALTER TABLE plans ALTER COLUMN plan_adjustment_limit SET NOT NULL;

-- categories: sort_order now has default, make NOT NULL
ALTER TABLE categories ALTER COLUMN sort_order SET NOT NULL;
