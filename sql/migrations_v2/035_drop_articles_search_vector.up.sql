-- Article full-text search now runs through the Meilisearch-backed search
-- microservice (fed by search_outbox), so the PostgreSQL search_vector column
-- and its GIN index are no longer needed.
DROP INDEX IF EXISTS idx_articles_search;
ALTER TABLE articles DROP COLUMN IF EXISTS search_vector;
