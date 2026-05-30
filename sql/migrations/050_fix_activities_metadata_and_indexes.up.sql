-- Fix activities.metadata: enforce NOT NULL with default to avoid three-valued logic
-- and align with all other JSONB columns in the schema.
UPDATE activities SET metadata = '{}'::jsonb WHERE metadata IS NULL;
ALTER TABLE activities ALTER COLUMN metadata SET NOT NULL;
ALTER TABLE activities ALTER COLUMN metadata SET DEFAULT '{}'::jsonb;

-- Drop the GIN index on activities.metadata if no JSONB containment queries exist.
-- All current activity queries filter on user_id, item_type, or created_at only.
-- The GIN index adds ~2-3x write overhead with no read benefit for today's workload.
DROP INDEX IF EXISTS idx_activities_metadata;
