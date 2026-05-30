-- Add missing indexes for common query patterns

-- goal_habit_relations: reverse lookup (which goals contain this habit?)
CREATE INDEX IF NOT EXISTS idx_goal_habit_relations_habit_id ON goal_habit_relations(habit_id);

-- user_subscriptions: plan-based queries for revenue aggregation
CREATE INDEX IF NOT EXISTS idx_user_subscriptions_plan_id ON user_subscriptions(plan_id);

-- notifications: pagination of all notifications per user
CREATE INDEX IF NOT EXISTS idx_notifications_user_created ON notifications(user_id, created_at DESC);

-- upgrade_events: funnel analytics per user
CREATE INDEX IF NOT EXISTS idx_upgrade_events_user_created ON upgrade_events(user_id, created_at DESC);

-- processed_events: TTL cleanup of old idempotency keys
CREATE INDEX IF NOT EXISTS idx_processed_events_processed_at ON processed_events(processed_at);

-- ai_coach_processed_events: TTL cleanup of old idempotency keys
CREATE INDEX IF NOT EXISTS idx_ai_coach_processed_events_processed_at ON ai_coach_processed_events(processed_at);

-- Drop redundant indexes already covered by composite indexes or unique constraints

-- Covered by idx_messages_conversation_created_at (conversation_id, created_at)
DROP INDEX IF EXISTS idx_messages_conversation_id;

-- Covered by idx_activities_user_created_at (user_id, created_at)
DROP INDEX IF EXISTS idx_activities_user_id;

-- Covered by implicit unique index on plans(code)
DROP INDEX IF EXISTS idx_plans_code;
