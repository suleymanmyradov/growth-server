-- Rollback: remove category_id foreign key from articles

-- Drop the index
DROP INDEX IF EXISTS idx_articles_category_id;

-- Drop the foreign key constraint
ALTER TABLE articles DROP CONSTRAINT IF EXISTS fk_articles_category;

-- Drop the category_id column
ALTER TABLE articles DROP COLUMN IF EXISTS category_id;

-- Restore original search vector trigger
CREATE OR REPLACE FUNCTION update_article_search_vector()
RETURNS TRIGGER AS $$
BEGIN
    NEW.search_vector := to_tsvector('english',
        COALESCE(NEW.title, '') || ' ' ||
        COALESCE(NEW.excerpt, '') || ' ' ||
        COALESCE(NEW.content, '') || ' ' ||
        COALESCE(NEW.category, '')
    );
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
