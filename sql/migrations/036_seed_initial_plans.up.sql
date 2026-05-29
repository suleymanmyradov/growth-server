INSERT INTO plans (
    code,
    name,
    description,
    price_monthly_cents,
    price_annual_cents,
    active_goal_limit,
    active_habit_limit,
    weekly_review_history_limit,
    plan_adjustment_limit,
    personalized_ai_enabled
) VALUES
(
    'free',
    'Free',
    'Start with the core accountability loop.',
    0,
    0,
    1,
    3,
    1,
    3,
    FALSE
),
(
    'pro',
    'Pro',
    'Unlock deeper personalization, history, and unlimited accountability plans.',
    900,
    7200,
    NULL,
    NULL,
    NULL,
    NULL,
    TRUE
)
ON CONFLICT (code) DO NOTHING;
