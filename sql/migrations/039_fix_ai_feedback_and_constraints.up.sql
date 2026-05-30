-- Fix ai_feedback.id to use time-sortable uuid_generate_v7() for index locality
ALTER TABLE ai_feedback ALTER COLUMN id SET DEFAULT uuid_generate_v7();

-- Fix ai_feedback.model sentinel: allow NULL instead of empty string
UPDATE ai_feedback SET model = NULL WHERE model = '';
ALTER TABLE ai_feedback ALTER COLUMN model DROP DEFAULT;
ALTER TABLE ai_feedback ALTER COLUMN model TYPE TEXT; -- already TEXT, just clearing default

-- Add XOR constraint to plan_adjustment_suggestions: exactly one of goal_id/habit_id must be set
ALTER TABLE plan_adjustment_suggestions
ADD CONSTRAINT chk_exactly_one_target
CHECK (
    (goal_id IS NOT NULL AND habit_id IS NULL) OR
    (goal_id IS NULL AND habit_id IS NOT NULL)
);

-- Fix user_subscriptions.plan_id missing ON DELETE rule
ALTER TABLE user_subscriptions DROP CONSTRAINT IF EXISTS user_subscriptions_plan_id_fkey;
ALTER TABLE user_subscriptions
ADD CONSTRAINT user_subscriptions_plan_id_fkey
FOREIGN KEY (plan_id) REFERENCES plans(id) ON DELETE RESTRICT;

-- Add completion_rate range check to weekly_reviews
ALTER TABLE weekly_reviews
ADD CONSTRAINT chk_completion_rate_range
CHECK (completion_rate >= 0 AND completion_rate <= 100);
