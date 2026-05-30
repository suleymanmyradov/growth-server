-- Revert pgvector additions
DROP INDEX IF EXISTS idx_articles_ai_metadata;
DROP INDEX IF EXISTS idx_articles_embedding;
ALTER TABLE articles DROP COLUMN IF EXISTS ai_metadata;
ALTER TABLE articles DROP COLUMN IF EXISTS embedding;
-- We intentionally keep the vector extension installed to avoid breaking other databases
