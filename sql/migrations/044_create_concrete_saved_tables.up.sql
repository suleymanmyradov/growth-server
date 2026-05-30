-- Replace polymorphic saved_items with concrete tables for proper FK integrity
-- The old saved_items table is kept temporarily for backward compatibility.

CREATE TABLE saved_articles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    article_id UUID NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(article_id, user_id)
);

CREATE TABLE saved_goals (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    goal_id UUID NOT NULL REFERENCES goals(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(goal_id, user_id)
);

CREATE TABLE saved_habits (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    habit_id UUID NOT NULL REFERENCES habits(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(habit_id, user_id)
);

-- Migrate existing data from saved_items polymorphic table
INSERT INTO saved_articles (article_id, user_id, created_at)
SELECT item_id, user_id, created_at FROM saved_items
WHERE item_type = 'article'::saved_item_type
ON CONFLICT (article_id, user_id) DO NOTHING;

INSERT INTO saved_goals (goal_id, user_id, created_at)
SELECT item_id, user_id, created_at FROM saved_items
WHERE item_type = 'goal'::saved_item_type
ON CONFLICT (goal_id, user_id) DO NOTHING;

INSERT INTO saved_habits (habit_id, user_id, created_at)
SELECT item_id, user_id, created_at FROM saved_items
WHERE item_type = 'habit'::saved_item_type
ON CONFLICT (habit_id, user_id) DO NOTHING;

-- Add indexes for common access patterns
CREATE INDEX idx_saved_articles_user_id ON saved_articles(user_id);
CREATE INDEX idx_saved_articles_created_at ON saved_articles(created_at DESC);
CREATE INDEX idx_saved_goals_user_id ON saved_goals(user_id);
CREATE INDEX idx_saved_goals_created_at ON saved_goals(created_at DESC);
CREATE INDEX idx_saved_habits_user_id ON saved_habits(user_id);
CREATE INDEX idx_saved_habits_created_at ON saved_habits(created_at DESC);

-- Deprecate old saved_items table
COMMENT ON TABLE saved_items IS 'DEPRECATED: Use saved_articles, saved_goals, saved_habits instead. Kept for backward compatibility during transition.';
