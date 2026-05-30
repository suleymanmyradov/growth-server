-- Revert activities.metadata to nullable without default
ALTER TABLE activities ALTER COLUMN metadata DROP NOT NULL;
ALTER TABLE activities ALTER COLUMN metadata DROP DEFAULT;

-- Recreate the GIN index (applications that start using JSONB containment queries should add this back)
CREATE INDEX IF NOT EXISTS idx_activities_metadata ON activities USING GIN(metadata);
