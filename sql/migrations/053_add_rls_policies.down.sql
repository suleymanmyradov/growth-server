-- Drop all RLS policies
DROP POLICY IF EXISTS habits_user_isolation ON habits;
DROP POLICY IF EXISTS goals_user_isolation ON goals;
DROP POLICY IF EXISTS goal_habit_relations_user_isolation ON goal_habit_relations;
DROP POLICY IF EXISTS check_ins_user_isolation ON check_ins;
DROP POLICY IF EXISTS activities_user_isolation ON activities;
DROP POLICY IF EXISTS conversations_user_isolation ON conversations;
DROP POLICY IF EXISTS messages_user_isolation ON messages;
DROP POLICY IF EXISTS notifications_user_isolation ON notifications;
DROP POLICY IF EXISTS user_settings_user_isolation ON user_settings;
DROP POLICY IF EXISTS user_coaching_profiles_user_isolation ON user_coaching_profiles;
DROP POLICY IF EXISTS weekly_reviews_user_isolation ON weekly_reviews;
DROP POLICY IF EXISTS plan_adjustment_suggestions_user_isolation ON plan_adjustment_suggestions;
DROP POLICY IF EXISTS user_subscriptions_user_isolation ON user_subscriptions;
DROP POLICY IF EXISTS saved_articles_user_isolation ON saved_articles;
DROP POLICY IF EXISTS saved_goals_user_isolation ON saved_goals;
DROP POLICY IF EXISTS saved_habits_user_isolation ON saved_habits;
DROP POLICY IF EXISTS reminder_queue_user_isolation ON reminder_queue;
DROP POLICY IF EXISTS ai_feedback_user_isolation ON ai_feedback;
DROP POLICY IF EXISTS upgrade_events_user_isolation ON upgrade_events;
DROP POLICY IF EXISTS profiles_user_isolation ON profiles;
DROP POLICY IF EXISTS article_shares_user_isolation ON article_shares;

-- Disable RLS on all tables
ALTER TABLE habits DISABLE ROW LEVEL SECURITY;
ALTER TABLE goals DISABLE ROW LEVEL SECURITY;
ALTER TABLE goal_habit_relations DISABLE ROW LEVEL SECURITY;
ALTER TABLE check_ins DISABLE ROW LEVEL SECURITY;
ALTER TABLE activities DISABLE ROW LEVEL SECURITY;
ALTER TABLE conversations DISABLE ROW LEVEL SECURITY;
ALTER TABLE messages DISABLE ROW LEVEL SECURITY;
ALTER TABLE notifications DISABLE ROW LEVEL SECURITY;
ALTER TABLE user_settings DISABLE ROW LEVEL SECURITY;
ALTER TABLE user_coaching_profiles DISABLE ROW LEVEL SECURITY;
ALTER TABLE weekly_reviews DISABLE ROW LEVEL SECURITY;
ALTER TABLE plan_adjustment_suggestions DISABLE ROW LEVEL SECURITY;
ALTER TABLE user_subscriptions DISABLE ROW LEVEL SECURITY;
ALTER TABLE saved_articles DISABLE ROW LEVEL SECURITY;
ALTER TABLE saved_goals DISABLE ROW LEVEL SECURITY;
ALTER TABLE saved_habits DISABLE ROW LEVEL SECURITY;
ALTER TABLE reminder_queue DISABLE ROW LEVEL SECURITY;
ALTER TABLE ai_feedback DISABLE ROW LEVEL SECURITY;
ALTER TABLE upgrade_events DISABLE ROW LEVEL SECURITY;
ALTER TABLE profiles DISABLE ROW LEVEL SECURITY;
ALTER TABLE article_shares DISABLE ROW LEVEL SECURITY;

-- Remove FORCE flag (no-op if already disabled, but clean)
ALTER TABLE habits NO FORCE ROW LEVEL SECURITY;
ALTER TABLE goals NO FORCE ROW LEVEL SECURITY;
ALTER TABLE goal_habit_relations NO FORCE ROW LEVEL SECURITY;
ALTER TABLE check_ins NO FORCE ROW LEVEL SECURITY;
ALTER TABLE activities NO FORCE ROW LEVEL SECURITY;
ALTER TABLE conversations NO FORCE ROW LEVEL SECURITY;
ALTER TABLE messages NO FORCE ROW LEVEL SECURITY;
ALTER TABLE notifications NO FORCE ROW LEVEL SECURITY;
ALTER TABLE user_settings NO FORCE ROW LEVEL SECURITY;
ALTER TABLE user_coaching_profiles NO FORCE ROW LEVEL SECURITY;
ALTER TABLE weekly_reviews NO FORCE ROW LEVEL SECURITY;
ALTER TABLE plan_adjustment_suggestions NO FORCE ROW LEVEL SECURITY;
ALTER TABLE user_subscriptions NO FORCE ROW LEVEL SECURITY;
ALTER TABLE saved_articles NO FORCE ROW LEVEL SECURITY;
ALTER TABLE saved_goals NO FORCE ROW LEVEL SECURITY;
ALTER TABLE saved_habits NO FORCE ROW LEVEL SECURITY;
ALTER TABLE reminder_queue NO FORCE ROW LEVEL SECURITY;
ALTER TABLE ai_feedback NO FORCE ROW LEVEL SECURITY;
ALTER TABLE upgrade_events NO FORCE ROW LEVEL SECURITY;
ALTER TABLE profiles NO FORCE ROW LEVEL SECURITY;
ALTER TABLE article_shares NO FORCE ROW LEVEL SECURITY;

-- Drop helper function
DROP FUNCTION IF EXISTS current_app_user_id();
