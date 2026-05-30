-- Enable Row-Level Security (RLS) for multi-tenant data isolation
-- All user-scoped tables get a uniform policy: users can only see/modify their own rows.

-- Helper function to read the current application user_id from the connection.
-- The application MUST execute `SET LOCAL app.current_user_id = '<uuid>';`
-- on every connection checkout before running queries.
CREATE OR REPLACE FUNCTION current_app_user_id()
RETURNS UUID AS $$
BEGIN
    RETURN nullif(current_setting('app.current_user_id', true), '')::UUID;
END;
$$ LANGUAGE plpgsql STABLE SECURITY DEFINER;

-- ============================================
-- Enable RLS on all user-scoped tables
-- ============================================
ALTER TABLE habits ENABLE ROW LEVEL SECURITY;
ALTER TABLE goals ENABLE ROW LEVEL SECURITY;
ALTER TABLE goal_habit_relations ENABLE ROW LEVEL SECURITY;
ALTER TABLE check_ins ENABLE ROW LEVEL SECURITY;
ALTER TABLE activities ENABLE ROW LEVEL SECURITY;
ALTER TABLE conversations ENABLE ROW LEVEL SECURITY;
ALTER TABLE messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE notifications ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_settings ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_coaching_profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE weekly_reviews ENABLE ROW LEVEL SECURITY;
ALTER TABLE plan_adjustment_suggestions ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_subscriptions ENABLE ROW LEVEL SECURITY;
ALTER TABLE saved_articles ENABLE ROW LEVEL SECURITY;
ALTER TABLE saved_goals ENABLE ROW LEVEL SECURITY;
ALTER TABLE saved_habits ENABLE ROW LEVEL SECURITY;
ALTER TABLE reminder_queue ENABLE ROW LEVEL SECURITY;
ALTER TABLE ai_feedback ENABLE ROW LEVEL SECURITY;
ALTER TABLE upgrade_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE article_shares ENABLE ROW LEVEL SECURITY;

-- ============================================
-- Create uniform policies for each table
-- ============================================

-- habits
CREATE POLICY habits_user_isolation ON habits
    FOR ALL
    USING (user_id = current_app_user_id())
    WITH CHECK (user_id = current_app_user_id());

-- goals
CREATE POLICY goals_user_isolation ON goals
    FOR ALL
    USING (user_id = current_app_user_id())
    WITH CHECK (user_id = current_app_user_id());

-- goal_habit_relations (join through goals)
CREATE POLICY goal_habit_relations_user_isolation ON goal_habit_relations
    FOR ALL
    USING (goal_id IN (SELECT id FROM goals WHERE user_id = current_app_user_id()))
    WITH CHECK (goal_id IN (SELECT id FROM goals WHERE user_id = current_app_user_id()));

-- check_ins
CREATE POLICY check_ins_user_isolation ON check_ins
    FOR ALL
    USING (user_id = current_app_user_id())
    WITH CHECK (user_id = current_app_user_id());

-- activities
CREATE POLICY activities_user_isolation ON activities
    FOR ALL
    USING (user_id = current_app_user_id())
    WITH CHECK (user_id = current_app_user_id());

-- conversations
CREATE POLICY conversations_user_isolation ON conversations
    FOR ALL
    USING (user_id = current_app_user_id())
    WITH CHECK (user_id = current_app_user_id());

-- messages (join through conversations)
CREATE POLICY messages_user_isolation ON messages
    FOR ALL
    USING (conversation_id IN (SELECT id FROM conversations WHERE user_id = current_app_user_id()))
    WITH CHECK (conversation_id IN (SELECT id FROM conversations WHERE user_id = current_app_user_id()));

-- notifications
CREATE POLICY notifications_user_isolation ON notifications
    FOR ALL
    USING (user_id = current_app_user_id())
    WITH CHECK (user_id = current_app_user_id());

-- user_settings
CREATE POLICY user_settings_user_isolation ON user_settings
    FOR ALL
    USING (user_id = current_app_user_id())
    WITH CHECK (user_id = current_app_user_id());

-- user_coaching_profiles
CREATE POLICY user_coaching_profiles_user_isolation ON user_coaching_profiles
    FOR ALL
    USING (user_id = current_app_user_id())
    WITH CHECK (user_id = current_app_user_id());

-- weekly_reviews
CREATE POLICY weekly_reviews_user_isolation ON weekly_reviews
    FOR ALL
    USING (user_id = current_app_user_id())
    WITH CHECK (user_id = current_app_user_id());

-- plan_adjustment_suggestions
CREATE POLICY plan_adjustment_suggestions_user_isolation ON plan_adjustment_suggestions
    FOR ALL
    USING (user_id = current_app_user_id())
    WITH CHECK (user_id = current_app_user_id());

-- user_subscriptions
CREATE POLICY user_subscriptions_user_isolation ON user_subscriptions
    FOR ALL
    USING (user_id = current_app_user_id())
    WITH CHECK (user_id = current_app_user_id());

-- saved_articles
CREATE POLICY saved_articles_user_isolation ON saved_articles
    FOR ALL
    USING (user_id = current_app_user_id())
    WITH CHECK (user_id = current_app_user_id());

-- saved_goals
CREATE POLICY saved_goals_user_isolation ON saved_goals
    FOR ALL
    USING (user_id = current_app_user_id())
    WITH CHECK (user_id = current_app_user_id());

-- saved_habits
CREATE POLICY saved_habits_user_isolation ON saved_habits
    FOR ALL
    USING (user_id = current_app_user_id())
    WITH CHECK (user_id = current_app_user_id());

-- reminder_queue
CREATE POLICY reminder_queue_user_isolation ON reminder_queue
    FOR ALL
    USING (user_id = current_app_user_id())
    WITH CHECK (user_id = current_app_user_id());

-- ai_feedback
CREATE POLICY ai_feedback_user_isolation ON ai_feedback
    FOR ALL
    USING (user_id = current_app_user_id())
    WITH CHECK (user_id = current_app_user_id());

-- upgrade_events
CREATE POLICY upgrade_events_user_isolation ON upgrade_events
    FOR ALL
    USING (user_id = current_app_user_id())
    WITH CHECK (user_id = current_app_user_id());

-- profiles
CREATE POLICY profiles_user_isolation ON profiles
    FOR ALL
    USING (user_id = current_app_user_id())
    WITH CHECK (user_id = current_app_user_id());

-- article_shares
CREATE POLICY article_shares_user_isolation ON article_shares
    FOR ALL
    USING (user_id = current_app_user_id())
    WITH CHECK (user_id = current_app_user_id());

-- ============================================
-- Force RLS for table owners (bypass otherwise)
-- ============================================
ALTER TABLE habits FORCE ROW LEVEL SECURITY;
ALTER TABLE goals FORCE ROW LEVEL SECURITY;
ALTER TABLE goal_habit_relations FORCE ROW LEVEL SECURITY;
ALTER TABLE check_ins FORCE ROW LEVEL SECURITY;
ALTER TABLE activities FORCE ROW LEVEL SECURITY;
ALTER TABLE conversations FORCE ROW LEVEL SECURITY;
ALTER TABLE messages FORCE ROW LEVEL SECURITY;
ALTER TABLE notifications FORCE ROW LEVEL SECURITY;
ALTER TABLE user_settings FORCE ROW LEVEL SECURITY;
ALTER TABLE user_coaching_profiles FORCE ROW LEVEL SECURITY;
ALTER TABLE weekly_reviews FORCE ROW LEVEL SECURITY;
ALTER TABLE plan_adjustment_suggestions FORCE ROW LEVEL SECURITY;
ALTER TABLE user_subscriptions FORCE ROW LEVEL SECURITY;
ALTER TABLE saved_articles FORCE ROW LEVEL SECURITY;
ALTER TABLE saved_goals FORCE ROW LEVEL SECURITY;
ALTER TABLE saved_habits FORCE ROW LEVEL SECURITY;
ALTER TABLE reminder_queue FORCE ROW LEVEL SECURITY;
ALTER TABLE ai_feedback FORCE ROW LEVEL SECURITY;
ALTER TABLE upgrade_events FORCE ROW LEVEL SECURITY;
ALTER TABLE profiles FORCE ROW LEVEL SECURITY;
ALTER TABLE article_shares FORCE ROW LEVEL SECURITY;
