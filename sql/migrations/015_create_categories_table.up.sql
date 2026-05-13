-- Categories table for centralized category management
-- Supports multiple entity types: articles, habits, goals

CREATE TYPE entity_type AS ENUM ('article', 'habit', 'goal');

CREATE TABLE categories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    name VARCHAR(50) NOT NULL,
    slug VARCHAR(50) NOT NULL,
    entity_type entity_type NOT NULL,
    sort_order INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(slug, entity_type)
);

CREATE INDEX idx_categories_entity_type ON categories(entity_type);
CREATE INDEX idx_categories_slug ON categories(slug);

-- Trigger to update updated_at timestamp
CREATE TRIGGER update_categories_updated_at 
    BEFORE UPDATE ON categories 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

-- Insert default categories for articles
INSERT INTO categories (name, slug, entity_type, sort_order) VALUES
    ('Productivity', 'productivity', 'article', 1),
    ('Wellness', 'wellness', 'article', 2),
    ('Learning', 'learning', 'article', 3),
    ('Philosophy', 'philosophy', 'article', 4),
    ('Habits', 'habits', 'article', 5),
    ('Relationships', 'relationships', 'article', 6);

-- Insert default categories for habits
INSERT INTO categories (name, slug, entity_type, sort_order) VALUES
    ('Wellness', 'wellness', 'habit', 1),
    ('Learning', 'learning', 'habit', 2),
    ('Fitness', 'fitness', 'habit', 3),
    ('Mindfulness', 'mindfulness', 'habit', 4),
    ('Productivity', 'productivity', 'habit', 5),
    ('Health', 'health', 'habit', 6);

-- Insert default categories for goals
INSERT INTO categories (name, slug, entity_type, sort_order) VALUES
    ('Fitness', 'fitness', 'goal', 1),
    ('Learning', 'learning', 'goal', 2),
    ('Wellness', 'wellness', 'goal', 3),
    ('Mindfulness', 'mindfulness', 'goal', 4),
    ('Career', 'career', 'goal', 5),
    ('Productivity', 'productivity', 'goal', 6),
    ('Health', 'health', 'goal', 7);
