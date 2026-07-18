-- Recreate the search_vector column and GIN index (reverts 035).
ALTER TABLE articles
    ADD COLUMN search_vector tsvector GENERATED ALWAYS AS (
        to_tsvector('english',
            title || ' ' || coalesce(excerpt, '') || ' ' || content || ' ' || author)
    ) STORED;

CREATE INDEX idx_articles_search ON articles USING gin (search_vector);
