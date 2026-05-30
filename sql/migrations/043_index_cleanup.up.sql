-- Add missing indexes for common query patterns and drop redundant ones

-- check_ins: local_date is used for timezone-aware "today" queries
CREATE INDEX IF NOT EXISTS idx_check_ins_user_local_date ON check_ins(user_id, local_date);

-- activities: calendar queries group by created_at date range (DATE(created_at) is STABLE on timestamptz)
CREATE INDEX IF NOT EXISTS idx_activities_user_date ON activities(user_id, created_at);

-- articles: feed queries by category + published date
CREATE INDEX IF NOT EXISTS idx_articles_category_published ON articles(category_id, published_at DESC);

-- saved_items: composite index replaces single-column ones for common access patterns
CREATE INDEX IF NOT EXISTS idx_saved_items_user_type_created ON saved_items(user_id, item_type, created_at DESC);

-- weekly_reviews: GIN index for JSONB mood/energy summary queries
CREATE INDEX IF NOT EXISTS idx_weekly_reviews_mood ON weekly_reviews USING GIN(mood_summary);
CREATE INDEX IF NOT EXISTS idx_weekly_reviews_energy ON weekly_reviews USING GIN(energy_summary);

-- Drop redundant indexes (unique constraints already create these)
DROP INDEX IF EXISTS idx_user_coaching_profiles_user_id;  -- UNIQUE(user_id) already indexed
DROP INDEX IF EXISTS idx_user_subscriptions_user_id;        -- UNIQUE(user_id) already indexed

-- Drop obsolete articles.category string index (column dropped in migration 041)
-- (idx_articles_category was already dropped in 041, listed here for completeness)
