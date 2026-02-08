DROP TRIGGER IF EXISTS update_articles_updated_at ON articles;
DROP TRIGGER IF EXISTS update_articles_search_vector ON articles;
DROP FUNCTION IF EXISTS update_article_search_vector();
DROP INDEX IF EXISTS idx_articles_search;
DROP INDEX IF EXISTS idx_articles_author;
DROP INDEX IF EXISTS idx_articles_published_at;
DROP INDEX IF EXISTS idx_articles_category;
DROP TABLE IF EXISTS articles;
