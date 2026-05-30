CREATE INDEX IF NOT EXISTS idx_habits_name_trgm ON habits USING gin (name gin_trgm_ops);
