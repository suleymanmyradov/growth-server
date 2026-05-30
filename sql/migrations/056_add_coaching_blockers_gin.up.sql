-- Add GIN index on user_coaching_profiles.common_blockers to support
-- queries like "find users who have 'lack_of_time' as a common blocker"
-- using the JSONB `?` operator.
CREATE INDEX IF NOT EXISTS idx_coaching_blockers ON user_coaching_profiles USING GIN(common_blockers);

-- Also add a GIN index on coaching_notes for future flexibility
CREATE INDEX IF NOT EXISTS idx_coaching_notes ON user_coaching_profiles USING GIN(coaching_notes);
