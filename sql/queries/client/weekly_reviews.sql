-- name: CreateWeeklyReview :one
INSERT INTO weekly_reviews (
    user_id,
    week_start,
    week_end,
    total_habits,
    completed_check_ins,
    missed_check_ins,
    completion_rate,
    best_day,
    hardest_day,
    top_blocker,
    mood_summary,
    energy_summary,
    habit_breakdown,
    ai_summary,
    suggested_adjustments,
    next_week_plan
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
ON CONFLICT (user_id, week_start)
DO UPDATE SET
    week_end = EXCLUDED.week_end,
    total_habits = EXCLUDED.total_habits,
    completed_check_ins = EXCLUDED.completed_check_ins,
    missed_check_ins = EXCLUDED.missed_check_ins,
    completion_rate = EXCLUDED.completion_rate,
    best_day = EXCLUDED.best_day,
    hardest_day = EXCLUDED.hardest_day,
    top_blocker = EXCLUDED.top_blocker,
    mood_summary = EXCLUDED.mood_summary,
    energy_summary = EXCLUDED.energy_summary,
    habit_breakdown = EXCLUDED.habit_breakdown,
    ai_summary = EXCLUDED.ai_summary,
    suggested_adjustments = EXCLUDED.suggested_adjustments,
    next_week_plan = EXCLUDED.next_week_plan,
    generated_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: GetWeeklyReview :one
SELECT * FROM weekly_reviews
WHERE user_id = $1 AND week_start = $2;

-- name: GetCurrentWeeklyReview :one
SELECT * FROM weekly_reviews
WHERE user_id = $1
ORDER BY week_start DESC
LIMIT 1;

-- name: ListWeeklyReviews :many
SELECT * FROM weekly_reviews
WHERE user_id = $1
ORDER BY week_start DESC
LIMIT $2 OFFSET $3;

-- name: CountWeeklyReviews :one
SELECT COUNT(*) FROM weekly_reviews
WHERE user_id = $1;

-- name: GetCheckInStatsForWeek :many
SELECT
    h.id AS habit_id,
    h.name AS habit_name,
    h.category AS habit_category,
    COUNT(ci.id) AS total_check_ins,
    COUNT(*) FILTER (WHERE ci.status = 'completed') AS completed_count,
    COUNT(*) FILTER (WHERE ci.status = 'missed') AS missed_count,
    ROUND(
        CASE
            WHEN COUNT(ci.id) = 0 THEN 0
            ELSE (COUNT(*) FILTER (WHERE ci.status = 'completed')::numeric / COUNT(ci.id)::numeric) * 100
        END,
        2
    )::numeric AS completion_rate,
    MAX(ci.created_at) AS last_check_in_at
FROM habits h
LEFT JOIN check_ins ci
    ON ci.habit_id = h.id
   AND ci.user_id = h.user_id
   AND ci.created_at >= $2
   AND ci.created_at < $3
WHERE h.user_id = $1
GROUP BY h.id, h.name, h.category
ORDER BY h.created_at DESC;

-- name: GetDailyCheckInStatsForWeek :many
-- NOTE: DATE() uses UTC. Week boundaries ($2/$3) are timezone-aware so no
-- check-ins outside the user's week are included, but best/hardest day
-- attribution may be off by one day near midnight for non-UTC users.
SELECT
    DATE(ci.created_at) AS day,
    COUNT(*) AS total_check_ins,
    COUNT(*) FILTER (WHERE ci.status = 'completed') AS completed_count,
    COUNT(*) FILTER (WHERE ci.status = 'missed') AS missed_count
FROM check_ins ci
WHERE ci.user_id = $1
  AND ci.created_at >= $2
  AND ci.created_at < $3
GROUP BY DATE(ci.created_at)
ORDER BY day ASC;

-- name: GetBlockerStatsForWeek :many
SELECT blocker::text AS blocker, COUNT(*) AS count
FROM check_ins
WHERE user_id = $1
  AND created_at >= $2
  AND created_at < $3
  AND status = 'missed'
  AND blocker IS NOT NULL
GROUP BY blocker
ORDER BY count DESC;

-- name: GetMoodStatsForWeek :many
SELECT mood::text AS mood, COUNT(*) AS count
FROM check_ins
WHERE user_id = $1
  AND created_at >= $2
  AND created_at < $3
  AND mood IS NOT NULL
GROUP BY mood
ORDER BY count DESC;

-- name: GetEnergyStatsForWeek :many
SELECT energy::text AS energy, COUNT(*) AS count
FROM check_ins
WHERE user_id = $1
  AND created_at >= $2
  AND created_at < $3
  AND energy IS NOT NULL
GROUP BY energy
ORDER BY count DESC;
