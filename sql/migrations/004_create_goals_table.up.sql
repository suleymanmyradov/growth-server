CREATE TABLE goals (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    title VARCHAR(200) NOT NULL,
    description TEXT,
    category VARCHAR(50) NOT NULL,
    due_date TIMESTAMP WITH TIME ZONE,
    progress INTEGER DEFAULT 0 CHECK (progress >= 0 AND progress <= 100),
    completed BOOLEAN DEFAULT FALSE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE goal_habit_relations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    goal_id UUID NOT NULL REFERENCES goals(id) ON DELETE CASCADE,
    habit_id UUID NOT NULL REFERENCES habits(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(goal_id, habit_id)
);

CREATE INDEX idx_goals_user_id ON goals(user_id);
CREATE INDEX idx_goals_category ON goals(category);
CREATE INDEX idx_goals_not_completed ON goals(user_id) WHERE completed = FALSE;
CREATE INDEX idx_goals_due_date ON goals(due_date);
CREATE INDEX idx_goal_habit_relations_goal_id ON goal_habit_relations(goal_id);
CREATE INDEX idx_goal_habit_relations_habit_id ON goal_habit_relations(habit_id);

-- Trigger to update updated_at timestamp
CREATE TRIGGER update_goals_updated_at 
    BEFORE UPDATE ON goals 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();
