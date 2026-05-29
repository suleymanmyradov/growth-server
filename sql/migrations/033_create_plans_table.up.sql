CREATE TABLE plans (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    code VARCHAR(30) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    price_monthly_cents INTEGER NOT NULL DEFAULT 0,
    price_annual_cents INTEGER NOT NULL DEFAULT 0,
    active_goal_limit INTEGER,
    active_habit_limit INTEGER,
    weekly_review_history_limit INTEGER,
    plan_adjustment_limit INTEGER,
    personalized_ai_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    stripe_monthly_price_id VARCHAR(255),
    stripe_annual_price_id VARCHAR(255),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_plans_code ON plans(code);
CREATE INDEX idx_plans_is_active ON plans(is_active);

CREATE TRIGGER update_plans_updated_at
    BEFORE UPDATE ON plans
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
