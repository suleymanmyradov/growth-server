-- Add pg_trgm for fuzzy string matching, typo tolerance, and substring search
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Fuzzy search on article titles (typo-tolerant autocomplete)
CREATE INDEX idx_articles_title_trgm ON articles USING gin (title gin_trgm_ops);

-- Fuzzy search on habit names (user-facing search/filter)
CREATE INDEX idx_habits_name_trgm ON habits USING gin (name gin_trgm_ops);

-- Fuzzy search on goal titles
CREATE INDEX idx_goals_title_trgm ON goals USING gin (title gin_trgm_ops);

-- Fuzzy search on article author names
CREATE INDEX idx_articles_author_trgm ON articles USING gin (author gin_trgm_ops);

-- Substring/accent-insensitive search on categories.name
CREATE INDEX idx_categories_name_trgm ON categories USING gin (name gin_trgm_ops);
