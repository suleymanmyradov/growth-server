-- Drop the obsolete string category column on articles (queries now use category_id FK)
-- First verify nothing references the old column via trigger, then drop index and column

DROP INDEX IF EXISTS idx_articles_category;
ALTER TABLE articles DROP COLUMN IF EXISTS category;

-- Fix search vector trigger: include author back (was lost in migration 016) and avoid redundant subquery
-- The category name is looked up once per row via indexed FK; this is acceptable.
-- We include author explicitly since it was dropped from the search vector in migration 016.
CREATE OR REPLACE FUNCTION update_article_search_vector()
RETURNS TRIGGER AS $$
BEGIN
    NEW.search_vector := to_tsvector('english',
        COALESCE(NEW.title, '') || ' ' ||
        COALESCE(NEW.excerpt, '') || ' ' ||
        COALESCE(NEW.content, '') || ' ' ||
        COALESCE((SELECT name FROM categories WHERE id = NEW.category_id), '') || ' ' ||
        COALESCE(NEW.author, '')
    );
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
