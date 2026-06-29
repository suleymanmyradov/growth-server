-- name: ListActivePlans :many
SELECT id, code, name, description, price_monthly_cents, price_annual_cents, currency, active_goal_limit, active_habit_limit, weekly_review_history_limit, plan_adjustment_limit, personalized_ai_enabled, stripe_monthly_price_id, stripe_annual_price_id, is_active, created_at, updated_at
FROM plans
WHERE is_active = TRUE
ORDER BY price_monthly_cents ASC;

-- name: GetPlanByCode :one
SELECT id, code, name, description, price_monthly_cents, price_annual_cents, currency, active_goal_limit, active_habit_limit, weekly_review_history_limit, plan_adjustment_limit, personalized_ai_enabled, stripe_monthly_price_id, stripe_annual_price_id, is_active, created_at, updated_at
FROM plans
WHERE code = $1 AND is_active = TRUE;

-- name: GetUserSubscription :one
SELECT
    s.id, s.user_id, s.plan_id, s.status, s.billing_interval, s.current_period_start, s.current_period_end, s.trial_end, s.cancel_at_period_end, s.stripe_customer_id, s.stripe_subscription_id, s.created_at, s.updated_at,
    p.code AS plan_code,
    p.name AS plan_name,
    p.active_goal_limit,
    p.active_habit_limit,
    p.weekly_review_history_limit,
    p.plan_adjustment_limit,
    p.personalized_ai_enabled
FROM subscriptions s
JOIN plans p ON p.id = s.plan_id
WHERE s.user_id = $1;

-- name: CreateDefaultFreeSubscription :one
INSERT INTO subscriptions (user_id, plan_id, status)
SELECT $1, p.id, 'free'
FROM plans p
WHERE p.code = 'free'
ON CONFLICT (user_id) DO UPDATE SET updated_at = now()
RETURNING id, user_id, plan_id, status, billing_interval, current_period_start, current_period_end, trial_end, cancel_at_period_end, stripe_customer_id, stripe_subscription_id, created_at, updated_at;

-- name: UpsertUserSubscription :one
INSERT INTO subscriptions (
    user_id,
    plan_id,
    status,
    billing_interval,
    current_period_start,
    current_period_end,
    trial_end,
    cancel_at_period_end,
    stripe_customer_id,
    stripe_subscription_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (user_id)
DO UPDATE SET
    plan_id = EXCLUDED.plan_id,
    status = EXCLUDED.status,
    billing_interval = EXCLUDED.billing_interval,
    current_period_start = EXCLUDED.current_period_start,
    current_period_end = EXCLUDED.current_period_end,
    trial_end = EXCLUDED.trial_end,
    cancel_at_period_end = EXCLUDED.cancel_at_period_end,
    stripe_customer_id = EXCLUDED.stripe_customer_id,
    stripe_subscription_id = EXCLUDED.stripe_subscription_id
RETURNING id, user_id, plan_id, status, billing_interval, current_period_start, current_period_end, trial_end, cancel_at_period_end, stripe_customer_id, stripe_subscription_id, created_at, updated_at;

-- name: CreateUpgradeEvent :one
WITH ins AS (
    INSERT INTO upgrade_events (
        user_id, event_type, surface, trigger_source, plan_id,
        billing_interval, feedback_reason, feedback_note, metadata
    )
    VALUES ($1, $2, $3, $4, (SELECT p2.id FROM plans p2 WHERE p2.code = $5), $6, $7, $8, $9)
    RETURNING id, user_id, plan_id, event_type, surface, trigger_source, billing_interval, feedback_reason, feedback_note, metadata, created_at
)
SELECT ins.id, ins.user_id, ins.plan_id, ins.event_type, ins.surface, ins.trigger_source, ins.billing_interval, ins.feedback_reason, ins.feedback_note, ins.metadata, ins.created_at, p.code AS plan_code
FROM ins
LEFT JOIN plans p ON p.id = ins.plan_id;

-- name: CountActiveGoalsForUser :one
SELECT COUNT(*) FROM goals
WHERE user_id = $1 AND status != 'completed';

-- name: CountActiveHabitsForUser :one
SELECT COUNT(*) FROM habits
WHERE user_id = $1;

-- name: CountPendingPlanAdjustmentsForUser :one
SELECT COUNT(*) FROM plan_adjustments
WHERE user_id = $1 AND status = 'pending';

-- name: GetUserSubscriptionByStripeCustomerID :one
SELECT
    s.id, s.user_id, s.plan_id, s.status, s.billing_interval, s.current_period_start, s.current_period_end, s.trial_end, s.cancel_at_period_end, s.stripe_customer_id, s.stripe_subscription_id, s.created_at, s.updated_at,
    p.code AS plan_code,
    p.name AS plan_name,
    p.active_goal_limit,
    p.active_habit_limit,
    p.weekly_review_history_limit,
    p.plan_adjustment_limit,
    p.personalized_ai_enabled
FROM subscriptions s
JOIN plans p ON p.id = s.plan_id
WHERE s.stripe_customer_id = $1;

-- name: IsStripeEventProcessed :one
SELECT EXISTS(
    SELECT 1 FROM processed_events
    WHERE consumer = 'stripe_webhooks' AND event_id = $1
);

-- name: MarkStripeEventProcessed :exec
INSERT INTO processed_events (consumer, event_id)
VALUES ('stripe_webhooks', $1)
ON CONFLICT DO NOTHING;

-- name: ListExpiredActiveSubscriptions :many
SELECT
    s.id, s.user_id, s.plan_id, s.status, s.billing_interval, s.current_period_start, s.current_period_end, s.trial_end, s.cancel_at_period_end, s.stripe_customer_id, s.stripe_subscription_id, s.created_at, s.updated_at,
    p.code AS plan_code,
    p.name AS plan_name,
    p.active_goal_limit,
    p.active_habit_limit,
    p.weekly_review_history_limit,
    p.plan_adjustment_limit,
    p.personalized_ai_enabled
FROM subscriptions s
JOIN plans p ON p.id = s.plan_id
WHERE s.status IN ('active', 'trialing')
  AND s.cancel_at_period_end = true
  AND s.current_period_end < NOW()
LIMIT $1;
