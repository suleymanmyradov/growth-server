CREATE TABLE plans (
    id                          uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    code                        varchar(30) NOT NULL UNIQUE,
    name                        varchar(100) NOT NULL,
    description                 text,
    price_monthly_cents         integer NOT NULL DEFAULT 0 CHECK (price_monthly_cents >= 0),
    price_annual_cents          integer NOT NULL DEFAULT 0 CHECK (price_annual_cents >= 0),
    currency                    varchar(3) NOT NULL DEFAULT 'usd',
    active_goal_limit           integer NOT NULL,
    active_habit_limit          integer NOT NULL,
    weekly_review_history_limit integer NOT NULL,
    plan_adjustment_limit       integer NOT NULL,
    personalized_ai_enabled     boolean NOT NULL DEFAULT false,
    stripe_monthly_price_id     varchar(255),
    stripe_annual_price_id      varchar(255),
    is_active                   boolean NOT NULL DEFAULT true,
    created_at                  timestamptz NOT NULL DEFAULT now(),
    updated_at                  timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER plans_set_updated_at
    BEFORE UPDATE ON plans
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Seed the two plans the app requires. The app lazily creates a default
-- 'free' subscription on first billing access, which needs the 'free' row.
INSERT INTO plans (
    code, name, description,
    price_monthly_cents, price_annual_cents,
    active_goal_limit, active_habit_limit,
    weekly_review_history_limit, plan_adjustment_limit,
    personalized_ai_enabled, is_active
) VALUES
    ('free', 'Free', 'Get started with the essentials.',
     0, 0, 3, 5, 1, 3, false, true),
    ('pro', 'Pro', 'Unlimited goals and habits, full history, and AI coaching.',
     999, 9990, 1000, 1000, 1000, 1000, true, true)
ON CONFLICT (code) DO NOTHING;
