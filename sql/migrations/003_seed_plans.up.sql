-- Seed the billing plans the application requires.
--
-- The app lazily creates a default 'free' subscription on first billing access
-- (CreateDefaultFreeSubscription: INSERT ... SELECT FROM plans WHERE code='free').
-- Without a 'free' plan row that INSERT matches zero rows and the :one query
-- returns ErrNoRows, surfacing as "failed to create subscription" (HTTP 500).
--
-- Pro bypasses numeric limits in ComputeEntitlements, but we still set generous
-- values so the limits remain meaningful if that logic changes.
-- Idempotent: safe to re-run (ON CONFLICT on the unique `code`).

INSERT INTO public.plans (
    code, name, description,
    price_monthly_cents, price_annual_cents,
    active_goal_limit, active_habit_limit,
    weekly_review_history_limit, plan_adjustment_limit,
    personalized_ai_enabled, is_active
) VALUES
    (
        'free', 'Free', 'Get started with the essentials.',
        0, 0,
        3, 5,
        1, 3,
        false, true
    ),
    (
        'pro', 'Pro', 'Unlimited goals and habits, full history, and AI coaching.',
        999, 9990,
        1000, 1000,
        1000, 1000,
        true, true
    )
ON CONFLICT (code) DO NOTHING;
