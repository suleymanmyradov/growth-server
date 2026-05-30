CREATE INDEX IF NOT EXISTS idx_articles_author_trgm ON articles USING gin (author gin_trgm_ops);
