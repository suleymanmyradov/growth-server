-- name: ListActivePlans :many
SELECT * FROM plans
WHERE is_active = TRUE
ORDER BY price_monthly_cents ASC;

-- name: GetPlanByCode :one
SELECT * FROM plans
WHERE code = $1 AND is_active = TRUE;

-- name: GetUserSubscription :one
SELECT
    us.*,
    p.code AS plan_code,
    p.name AS plan_name,
    p.active_goal_limit,
    p.active_habit_limit,
    p.weekly_review_history_limit,
    p.plan_adjustment_limit,
    p.personalized_ai_enabled
FROM user_subscriptions us
JOIN plans p ON p.id = us.plan_id
WHERE us.user_id = $1;

-- name: CreateDefaultFreeSubscription :one
INSERT INTO user_subscriptions (user_id, plan_id, status)
SELECT $1, p.id, 'free'
FROM plans p
WHERE p.code = 'free'
ON CONFLICT (user_id) DO UPDATE SET updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: UpsertUserSubscription :one
INSERT INTO user_subscriptions (
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
    stripe_subscription_id = EXCLUDED.stripe_subscription_id,
    updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: CreateUpgradeEvent :one
INSERT INTO upgrade_events (
    user_id,
    event_type,
    surface,
    trigger,
    plan_code,
    billing_interval,
    feedback_reason,
    feedback_note,
    metadata
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: CountActiveGoalsForUser :one
SELECT COUNT(*) FROM goals
WHERE user_id = $1 AND status != 'completed';

-- name: CountActiveHabitsForUser :one
SELECT COUNT(*) FROM habits
WHERE user_id = $1;

-- name: CountPendingPlanAdjustmentsForUser :one
SELECT COUNT(*) FROM plan_adjustment_suggestions
WHERE user_id = $1 AND status = 'pending';

-- name: GetUserSubscriptionByStripeCustomerID :one
SELECT
    us.id, us.user_id, us.plan_id, us.status, us.billing_interval, us.current_period_start, us.current_period_end, us.trial_end, us.cancel_at_period_end, us.stripe_customer_id, us.stripe_subscription_id, us.created_at, us.updated_at,
    p.code AS plan_code,
    p.name AS plan_name,
    p.active_goal_limit,
    p.active_habit_limit,
    p.weekly_review_history_limit,
    p.plan_adjustment_limit,
    p.personalized_ai_enabled
FROM user_subscriptions us
JOIN plans p ON p.id = us.plan_id
WHERE us.stripe_customer_id = $1;
