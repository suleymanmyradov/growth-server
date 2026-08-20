-- Read-only analytics queries for the adminway metrics endpoints.
-- These query the rollup tables owned and written by analytics-consumer.

-- name: GetDailyMetrics :many
SELECT metric_date, metric_name, metric_value, metadata, updated_at
FROM daily_metrics
WHERE ($1::date IS NULL OR metric_date >= $1)
  AND ($2::date IS NULL OR metric_date <= $2)
ORDER BY metric_date DESC, metric_name;

-- name: GetLifecycleCounts :many
SELECT event_type, COUNT(DISTINCT user_id)::bigint AS count
FROM user_lifecycle_events
WHERE ($1::timestamptz IS NULL OR occurred_at >= $1)
  AND ($2::timestamptz IS NULL OR occurred_at <= $2)
GROUP BY event_type
ORDER BY event_type;

-- name: GetLifecycleCountsByDay :many
SELECT occurred_at::date AS metric_date, event_type, COUNT(DISTINCT user_id)::bigint AS count
FROM user_lifecycle_events
WHERE ($1::date IS NULL OR occurred_at::date >= $1)
  AND ($2::date IS NULL OR occurred_at::date <= $2)
GROUP BY occurred_at::date, event_type
ORDER BY metric_date DESC, event_type;

-- name: GetConversionFunnelCounts :many
SELECT stage, COUNT(DISTINCT user_id)::bigint AS count
FROM conversion_funnels
WHERE ($1::timestamptz IS NULL OR entered_at >= $1)
  AND ($2::timestamptz IS NULL OR entered_at <= $2)
GROUP BY stage
ORDER BY stage;

-- name: GetRetentionCohorts :many
SELECT cohort_date, cohort_size, period_days, retained_count, retention_rate, updated_at
FROM retention_cohorts
WHERE ($1::date IS NULL OR cohort_date >= $1)
  AND ($2::date IS NULL OR cohort_date <= $2)
ORDER BY cohort_date DESC, period_days;

-- name: GetActivationRate :one
-- Activation = users who completed onboarding AND created at least one habit
-- within 24h of onboarding. Returns (activated, total_onboarded) for the period.
SELECT
    COUNT(DISTINCT CASE WHEN le_h.user_id IS NOT NULL THEN le_o.user_id END)::bigint AS activated,
    COUNT(DISTINCT le_o.user_id)::bigint AS total_onboarded
FROM user_lifecycle_events le_o
LEFT JOIN user_lifecycle_events le_h
    ON le_h.user_id = le_o.user_id
    AND le_h.event_type = 'first_habit'
    AND le_h.occurred_at <= le_o.occurred_at + INTERVAL '24 hours'
WHERE le_o.event_type = 'onboarding_completed'
  AND ($1::timestamptz IS NULL OR le_o.occurred_at >= $1)
  AND ($2::timestamptz IS NULL OR le_o.occurred_at <= $2);
