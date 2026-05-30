-- Re-add the string category column (data cannot be recovered; backfill from category_id if possible)
ALTER TABLE articles ADD COLUMN category VARCHAR(50);

-- Backfill from categories join
UPDATE articles a
SET category = COALESCE(c.name, 'Uncategorized')
FROM categories c
WHERE a.category_id = c.id;

CREATE INDEX idx_articles_category ON articles(category);

-- Restore original search vector trigger (without author, since that was the original 016 version)
CREATE OR REPLACE FUNCTION update_article_search_vector()
RETURNS TRIGGER AS $$
BEGIN
    NEW.search_vector := to_tsvector('english',
        COALESCE(NEW.title, '') || ' ' ||
        COALESCE(NEW.excerpt, '') || ' ' ||
        COALESCE(NEW.content, '') || ' ' ||
        COALESCE((SELECT name FROM categories WHERE id = NEW.category_id), '')
    );
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
