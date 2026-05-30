-- Revert index changes
DROP INDEX CONCURRENTLY IF EXISTS idx_check_ins_user_local_date;
DROP INDEX CONCURRENTLY IF EXISTS idx_activities_user_date;
DROP INDEX CONCURRENTLY IF EXISTS idx_articles_category_published;
DROP INDEX CONCURRENTLY IF EXISTS idx_saved_items_user_type_created;
DROP INDEX CONCURRENTLY IF EXISTS idx_weekly_reviews_mood;
DROP INDEX CONCURRENTLY IF EXISTS idx_weekly_reviews_energy;

-- Re-add redundant indexes
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_user_coaching_profiles_user_id ON user_coaching_profiles(user_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_user_subscriptions_user_id ON user_subscriptions(user_id);
