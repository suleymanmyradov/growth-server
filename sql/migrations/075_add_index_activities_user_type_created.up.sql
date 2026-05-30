-- Composite index for GetActivityFeed / ListActivitiesByType
CREATE INDEX IF NOT EXISTS idx_activities_user_type_created_desc ON activities(user_id, item_type, created_at DESC);
