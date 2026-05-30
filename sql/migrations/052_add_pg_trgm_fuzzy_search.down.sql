DROP INDEX IF EXISTS idx_categories_name_trgm;
DROP INDEX IF EXISTS idx_articles_author_trgm;
DROP INDEX IF EXISTS idx_goals_title_trgm;
DROP INDEX IF EXISTS idx_habits_name_trgm;
DROP INDEX IF EXISTS idx_articles_title_trgm;
-- pg_trgm extension is left installed; dropping it can break other indexes
