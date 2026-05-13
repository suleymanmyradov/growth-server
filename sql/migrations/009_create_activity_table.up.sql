CREATE TABLE activities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    item_type VARCHAR(30) NOT NULL CHECK (item_type IN ('habit_completed', 'goal_created', 'goal_completed', 'article_saved')),
    title VARCHAR(200) NOT NULL,
    description TEXT,
    metadata JSONB, -- Additional data as JSON
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_activities_user_id ON activities(user_id);
CREATE INDEX idx_activities_item_type ON activities(item_type);
CREATE INDEX idx_activities_created_at ON activities(created_at);
CREATE INDEX idx_activities_user_created_at ON activities(user_id, created_at);
CREATE INDEX idx_activities_metadata ON activities USING GIN(metadata);
