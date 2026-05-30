-- Revert changes
ALTER TABLE ai_feedback ALTER COLUMN id SET DEFAULT gen_random_uuid();
ALTER TABLE ai_feedback ALTER COLUMN model SET DEFAULT '';

ALTER TABLE plan_adjustment_suggestions DROP CONSTRAINT IF EXISTS chk_exactly_one_target;

ALTER TABLE user_subscriptions DROP CONSTRAINT IF EXISTS user_subscriptions_plan_id_fkey;
ALTER TABLE user_subscriptions
ADD CONSTRAINT user_subscriptions_plan_id_fkey
FOREIGN KEY (plan_id) REFERENCES plans(id);

ALTER TABLE weekly_reviews DROP CONSTRAINT IF EXISTS chk_completion_rate_range;
