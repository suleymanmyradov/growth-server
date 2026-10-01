-- name: ListActivePlans :many
SELECT id, code, name, description, price_monthly_cents, price_annual_cents, currency, active_goal_limit, active_habit_limit, weekly_review_history_limit, plan_adjustment_limit, personalized_ai_enabled, is_active, created_at, updated_at
FROM plans
WHERE is_active = TRUE
ORDER BY price_monthly_cents ASC;

-- name: GetPlanByCode :one
SELECT id, code, name, description, price_monthly_cents, price_annual_cents, currency, active_goal_limit, active_habit_limit, weekly_review_history_limit, plan_adjustment_limit, personalized_ai_enabled, is_active, created_at, updated_at
FROM plans
WHERE code = $1 AND is_active = TRUE;

-- name: GetUserSubscription :one
SELECT
    s.id, s.user_id, s.plan_id, s.status, s.billing_interval, s.current_period_start, s.current_period_end, s.trial_end, s.cancel_at_period_end, s.revenuecat_customer_id, s.paddle_customer_id, s.paddle_subscription_id, s.created_at, s.updated_at,
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

-- name: ApplyMergedSubscription :one
-- Single writer for the shared subscriptions row. The row is a merged
-- projection of subscription_provider_states: the caller picks the winning
-- provider state and passes the merged fields + provider link columns.
INSERT INTO subscriptions (
    user_id,
    plan_id,
    status,
    billing_interval,
    current_period_start,
    current_period_end,
    trial_end,
    cancel_at_period_end,
    paddle_customer_id,
    paddle_subscription_id,
    revenuecat_customer_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (user_id)
DO UPDATE SET
    plan_id = EXCLUDED.plan_id,
    status = EXCLUDED.status,
    billing_interval = EXCLUDED.billing_interval,
    current_period_start = EXCLUDED.current_period_start,
    current_period_end = EXCLUDED.current_period_end,
    trial_end = EXCLUDED.trial_end,
    cancel_at_period_end = EXCLUDED.cancel_at_period_end,
    paddle_customer_id = COALESCE(EXCLUDED.paddle_customer_id, subscriptions.paddle_customer_id),
    paddle_subscription_id = COALESCE(EXCLUDED.paddle_subscription_id, subscriptions.paddle_subscription_id),
    revenuecat_customer_id = COALESCE(EXCLUDED.revenuecat_customer_id, subscriptions.revenuecat_customer_id)
RETURNING *;

-- ─── Provider states ──────────────────────────────────────────────────────
-- One row per (user, provider): the last state each billing provider applied
-- plus the provider-side event watermark used to drop out-of-order webhooks.
-- The merged subscriptions row is recomputed from these rows — never let a
-- provider's webhook write the shared row directly.

-- name: GetSubscriptionProviderState :one
SELECT user_id, provider, status, billing_interval, current_period_start, current_period_end, trial_end, cancel_at_period_end, provider_customer_id, provider_subscription_id, last_event_at, last_event_id, created_at, updated_at
FROM subscription_provider_states
WHERE user_id = $1 AND provider = $2;

-- name: ListSubscriptionProviderStates :many
SELECT user_id, provider, status, billing_interval, current_period_start, current_period_end, trial_end, cancel_at_period_end, provider_customer_id, provider_subscription_id, last_event_at, last_event_id, created_at, updated_at
FROM subscription_provider_states
WHERE user_id = $1;

-- name: UpsertSubscriptionProviderState :one
-- last_event_at only ever moves forward (GREATEST ignores NULLs): an event
-- without a timestamp applies but doesn't lower the watermark.
INSERT INTO subscription_provider_states (
    user_id,
    provider,
    status,
    billing_interval,
    current_period_start,
    current_period_end,
    trial_end,
    cancel_at_period_end,
    provider_customer_id,
    provider_subscription_id,
    last_event_at,
    last_event_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
ON CONFLICT (user_id, provider)
DO UPDATE SET
    status = EXCLUDED.status,
    billing_interval = EXCLUDED.billing_interval,
    current_period_start = EXCLUDED.current_period_start,
    current_period_end = EXCLUDED.current_period_end,
    trial_end = EXCLUDED.trial_end,
    cancel_at_period_end = EXCLUDED.cancel_at_period_end,
    provider_customer_id = COALESCE(EXCLUDED.provider_customer_id, subscription_provider_states.provider_customer_id),
    provider_subscription_id = COALESCE(EXCLUDED.provider_subscription_id, subscription_provider_states.provider_subscription_id),
    last_event_at = GREATEST(subscription_provider_states.last_event_at, EXCLUDED.last_event_at),
    last_event_id = COALESCE(EXCLUDED.last_event_id, subscription_provider_states.last_event_id)
RETURNING *;

-- name: LinkPaddleProviderIDs :exec
-- Attaches Paddle customer/subscription IDs without touching provider state —
-- used by transaction.completed, which carries IDs but no subscription entity.
INSERT INTO subscription_provider_states (
    user_id, provider, status, provider_customer_id, provider_subscription_id, last_event_at
)
VALUES ($1, 'paddle', 'free', $2, $3, $4)
ON CONFLICT (user_id, provider)
DO UPDATE SET
    provider_customer_id = COALESCE(EXCLUDED.provider_customer_id, subscription_provider_states.provider_customer_id),
    provider_subscription_id = COALESCE(EXCLUDED.provider_subscription_id, subscription_provider_states.provider_subscription_id),
    last_event_at = GREATEST(subscription_provider_states.last_event_at, EXCLUDED.last_event_at);

-- ─── Recorded checkouts ───────────────────────────────────────────────────
-- CreatePaddleCheckout records transaction_id -> user_id here. The webhook
-- trusts custom_data.user_id only for transactions we created server-side —
-- client-side Paddle.js checkouts leave custom_data attacker-controlled.

-- name: RecordPaddleCheckout :exec
INSERT INTO paddle_checkouts (transaction_id, user_id)
VALUES ($1, $2)
ON CONFLICT (transaction_id) DO NOTHING;

-- name: GetPaddleCheckoutUserID :one
SELECT user_id FROM paddle_checkouts WHERE transaction_id = $1;

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

-- name: ListSubscriptionStatuses :many
-- Admin broadcast audience segmentation: returns every user's plan code +
-- subscription status. adminway classifies users as premium (status in
-- active/trialing AND plan_code != 'free') vs free (everyone else).
SELECT s.user_id, p.code AS plan_code, s.status
FROM subscriptions s
JOIN plans p ON p.id = s.plan_id
ORDER BY s.user_id;

-- ─── Paddle queries ─────────────────────────────────────────────────────────
-- Paddle webhooks deliver subscription changes for web checkout. See migration
-- 058 and pkg/paddle.

-- name: GetUserSubscriptionByPaddleCustomerID :one
SELECT
    s.id, s.user_id, s.plan_id, s.status, s.billing_interval, s.current_period_start, s.current_period_end, s.trial_end, s.cancel_at_period_end, s.revenuecat_customer_id, s.paddle_customer_id, s.paddle_subscription_id, s.created_at, s.updated_at,
    p.code AS plan_code,
    p.name AS plan_name,
    p.active_goal_limit,
    p.active_habit_limit,
    p.weekly_review_history_limit,
    p.plan_adjustment_limit,
    p.personalized_ai_enabled
FROM subscriptions s
JOIN plans p ON p.id = s.plan_id
WHERE s.paddle_customer_id = $1;



-- name: IsPaddleEventProcessed :one
SELECT EXISTS(
    SELECT 1 FROM billing_webhook_events
    WHERE consumer = 'paddle_webhooks' AND event_id = $1
);

-- name: MarkPaddleEventProcessed :exec
INSERT INTO billing_webhook_events (consumer, event_id)
VALUES ('paddle_webhooks', $1)
ON CONFLICT DO NOTHING;

-- ─── RevenueCat queries ─────────────────────────────────────────────────────
-- RevenueCat webhooks deliver entitlement changes from App Store / Play Store.
-- See migration 043 and docs/push-notifications-design.md (billing section).

-- name: GetUserSubscriptionByRevenueCatCustomerID :one
SELECT
    s.id, s.user_id, s.plan_id, s.status, s.billing_interval, s.current_period_start, s.current_period_end, s.trial_end, s.cancel_at_period_end, s.revenuecat_customer_id, s.paddle_customer_id, s.paddle_subscription_id, s.created_at, s.updated_at,
    p.code AS plan_code,
    p.name AS plan_name,
    p.active_goal_limit,
    p.active_habit_limit,
    p.weekly_review_history_limit,
    p.plan_adjustment_limit,
    p.personalized_ai_enabled
FROM subscriptions s
JOIN plans p ON p.id = s.plan_id
WHERE s.revenuecat_customer_id = $1;

-- name: GetUserSubscriptionByUserID :one
-- Used by the RevenueCat webhook handler to look up the subscription by user
-- UUID (RevenueCat's app_user_id after Purchases.logIn).
SELECT
    s.id, s.user_id, s.plan_id, s.status, s.billing_interval, s.current_period_start, s.current_period_end, s.trial_end, s.cancel_at_period_end, s.revenuecat_customer_id, s.paddle_customer_id, s.paddle_subscription_id, s.created_at, s.updated_at,
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

-- name: IsRevenueCatEventProcessed :one
SELECT EXISTS(
    SELECT 1 FROM billing_webhook_events
    WHERE consumer = 'revenuecat_webhooks' AND event_id = $1
);

-- name: MarkRevenueCatEventProcessed :exec
INSERT INTO billing_webhook_events (consumer, event_id)
VALUES ('revenuecat_webhooks', $1)
ON CONFLICT DO NOTHING;
