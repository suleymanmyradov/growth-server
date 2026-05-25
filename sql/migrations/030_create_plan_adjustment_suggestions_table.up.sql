CREATE TABLE plan_adjustment_suggestions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    goal_id UUID REFERENCES goals(id) ON DELETE CASCADE,
    habit_id UUID REFERENCES habits(id) ON DELETE CASCADE,
    source VARCHAR(30) NOT NULL
        CHECK (source IN ('check_in', 'weekly_review', 'assistant', 'pattern_analysis')),
    adjustment_type VARCHAR(30) NOT NULL
        CHECK (adjustment_type IN ('reduce_difficulty', 'increase_difficulty', 'change_time', 'clarify_plan', 'pause', 'keep_same')),
    reason TEXT NOT NULL,
    suggestion TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'accepted', 'dismissed', 'applied')),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_plan_adjustment_suggestions_user_id ON plan_adjustment_suggestions(user_id);
CREATE INDEX idx_plan_adjustment_suggestions_habit_id ON plan_adjustment_suggestions(habit_id);
CREATE INDEX idx_plan_adjustment_suggestions_status ON plan_adjustment_suggestions(status);

CREATE TRIGGER update_plan_adjustment_suggestions_updated_at
    BEFORE UPDATE ON plan_adjustment_suggestions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();