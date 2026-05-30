-- Re-add redundant indexes
CREATE INDEX IF NOT EXISTS idx_messages_conversation_id ON messages(conversation_id);
CREATE INDEX IF NOT EXISTS idx_activities_user_id ON activities(user_id);
CREATE INDEX IF NOT EXISTS idx_plans_code ON plans(code);

-- Drop new indexes
DROP INDEX IF EXISTS idx_goal_habit_relations_habit_id;
DROP INDEX IF EXISTS idx_user_subscriptions_plan_id;
DROP INDEX IF EXISTS idx_notifications_user_created;
DROP INDEX IF EXISTS idx_upgrade_events_user_created;
DROP INDEX IF EXISTS idx_processed_events_processed_at;
DROP INDEX IF EXISTS idx_ai_coach_processed_events_processed_at;
