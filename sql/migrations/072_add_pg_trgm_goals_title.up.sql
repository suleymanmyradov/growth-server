CREATE INDEX IF NOT EXISTS idx_goals_title_trgm ON goals USING gin (title gin_trgm_ops);
