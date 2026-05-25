CREATE TABLE weekly_reviews (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    week_start DATE NOT NULL,
    week_end DATE NOT NULL CHECK (week_end > week_start),
    total_habits INTEGER NOT NULL DEFAULT 0,
    completed_check_ins INTEGER NOT NULL DEFAULT 0,
    missed_check_ins INTEGER NOT NULL DEFAULT 0,
    completion_rate NUMERIC(5,2) NOT NULL DEFAULT 0,
    best_day VARCHAR(20),
    hardest_day VARCHAR(20),
    top_blocker VARCHAR(50),
    mood_summary JSONB NOT NULL DEFAULT '{}'::jsonb,
    energy_summary JSONB NOT NULL DEFAULT '{}'::jsonb,
    habit_breakdown JSONB NOT NULL DEFAULT '[]'::jsonb,
    ai_summary TEXT,
    suggested_adjustments JSONB NOT NULL DEFAULT '[]'::jsonb,
    next_week_plan JSONB NOT NULL DEFAULT '{}'::jsonb,
    generated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, week_start)
);

CREATE INDEX idx_weekly_reviews_user_week ON weekly_reviews(user_id, week_start DESC);

CREATE TRIGGER update_weekly_reviews_updated_at
    BEFORE UPDATE ON weekly_reviews
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
