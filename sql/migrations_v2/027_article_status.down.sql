DROP INDEX IF EXISTS idx_articles_published_status;
DROP INDEX IF EXISTS idx_articles_status;
ALTER TABLE articles DROP COLUMN IF EXISTS status;
