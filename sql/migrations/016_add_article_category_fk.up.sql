-- Add category_id foreign key to articles table
-- Step 1: Add category_id column
ALTER TABLE articles ADD COLUMN category_id UUID;

-- Step 2: Populate category_id by matching existing category string to categories.slug
-- First try to match by slug (lowercase), then by name
UPDATE articles a
SET category_id = c.id
FROM categories c
WHERE c.entity_type = 'article'
  AND (LOWER(a.category) = c.slug OR LOWER(a.category) = LOWER(c.name));

-- Step 3: Add foreign key constraint
ALTER TABLE articles 
ADD CONSTRAINT fk_articles_category 
FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE SET NULL;

-- Step 4: Create index for efficient joins
CREATE INDEX idx_articles_category_id ON articles(category_id);

-- Step 5: Drop old category column (optional - keeping for backward compatibility during transition)
-- ALTER TABLE articles DROP COLUMN category;

-- Step 6: Update search vector trigger to use joined category name
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
