-- name: ListActivePlans :many
SELECT * FROM plans
WHERE is_active = TRUE
ORDER BY price_monthly_cents ASC;

-- name: GetPlanByCode :one
SELECT * FROM plans
WHERE code = $1 AND is_active = TRUE;

-- name: GetUserSubscription :one
SELECT
    s.*,
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
RETURNING *;

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
RETURNING *;

-- name: CreateUpgradeEvent :one
WITH ins AS (
    INSERT INTO upgrade_events (
        user_id, event_type, surface, trigger_source, plan_id,
        billing_interval, feedback_reason, feedback_note, metadata
    )
    VALUES ($1, $2, $3, $4, (SELECT p2.id FROM plans p2 WHERE p2.code = $5), $6, $7, $8, $9)
    RETURNING *
)
SELECT ins.*, p.code AS plan_code
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
    s.*,
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
    s.*,
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
