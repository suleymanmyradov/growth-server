CREATE TABLE habits (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    name VARCHAR(100) NOT NULL,
    description TEXT,
    streak INTEGER DEFAULT 0,
    completed BOOLEAN DEFAULT FALSE,
    category VARCHAR(50) NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_habits_user_id ON habits(user_id);
CREATE INDEX idx_habits_category ON habits(category);
CREATE INDEX idx_habits_not_completed ON habits(user_id) WHERE completed = FALSE;

-- Trigger to update updated_at timestamp
CREATE TRIGGER update_habits_updated_at 
    BEFORE UPDATE ON habits 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();
