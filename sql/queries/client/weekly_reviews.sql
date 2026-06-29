-- week_end is derived (week_start + 6) and generated_at is just updated_at;
-- both are exposed under their old names for callers.

-- name: CreateWeeklyReview :one
WITH ins AS (
    INSERT INTO weekly_reviews (
        user_id,
        week_start,
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
    VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
    ON CONFLICT (user_id, week_start)
    DO UPDATE SET
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
        updated_at = now()
    RETURNING id, user_id, week_start, total_habits, completed_check_ins, missed_check_ins, completion_rate, best_day, hardest_day, top_blocker, mood_summary, energy_summary, habit_breakdown, ai_summary, suggested_adjustments, next_week_plan, created_at, updated_at
)
SELECT ins.id, ins.user_id, ins.week_start, ins.total_habits, ins.completed_check_ins, ins.missed_check_ins, ins.completion_rate, ins.best_day, ins.hardest_day, ins.top_blocker, ins.mood_summary, ins.energy_summary, ins.habit_breakdown, ins.ai_summary, ins.suggested_adjustments, ins.next_week_plan, ins.created_at, ins.updated_at, (ins.week_start + 6)::date AS week_end, ins.updated_at AS generated_at
FROM ins;

-- name: GetWeeklyReview :one
SELECT wr.id, wr.user_id, wr.week_start, wr.total_habits, wr.completed_check_ins, wr.missed_check_ins, wr.completion_rate, wr.best_day, wr.hardest_day, wr.top_blocker, wr.mood_summary, wr.energy_summary, wr.habit_breakdown, wr.ai_summary, wr.suggested_adjustments, wr.next_week_plan, wr.created_at, wr.updated_at, (wr.week_start + 6)::date AS week_end, wr.updated_at AS generated_at
FROM weekly_reviews wr
WHERE wr.user_id = $1 AND wr.week_start = $2;

-- name: GetCurrentWeeklyReview :one
SELECT wr.id, wr.user_id, wr.week_start, wr.total_habits, wr.completed_check_ins, wr.missed_check_ins, wr.completion_rate, wr.best_day, wr.hardest_day, wr.top_blocker, wr.mood_summary, wr.energy_summary, wr.habit_breakdown, wr.ai_summary, wr.suggested_adjustments, wr.next_week_plan, wr.created_at, wr.updated_at, (wr.week_start + 6)::date AS week_end, wr.updated_at AS generated_at
FROM weekly_reviews wr
WHERE wr.user_id = $1
ORDER BY wr.week_start DESC
LIMIT 1;

-- name: ListWeeklyReviews :many
SELECT wr.id, wr.user_id, wr.week_start, wr.total_habits, wr.completed_check_ins, wr.missed_check_ins, wr.completion_rate, wr.best_day, wr.hardest_day, wr.top_blocker, wr.mood_summary, wr.energy_summary, wr.habit_breakdown, wr.ai_summary, wr.suggested_adjustments, wr.next_week_plan, wr.created_at, wr.updated_at, (wr.week_start + 6)::date AS week_end, wr.updated_at AS generated_at
FROM weekly_reviews wr
WHERE wr.user_id = $1
ORDER BY wr.week_start DESC
LIMIT $2 OFFSET $3;

-- name: CountWeeklyReviews :one
SELECT COUNT(*) FROM weekly_reviews
WHERE user_id = $1;

-- name: GetCheckInStatsForWeek :many
SELECT
    h.id AS habit_id,
    h.name AS habit_name,
    COALESCE(c.slug, '')::varchar AS habit_category,
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
LEFT JOIN categories c ON c.id = h.category_id
LEFT JOIN check_ins ci
    ON ci.habit_id = h.id
   AND ci.user_id = h.user_id
   AND ci.local_date >= $2
   AND ci.local_date < $3
WHERE h.user_id = $1
GROUP BY h.id, h.name, c.slug, h.created_at
ORDER BY h.created_at DESC;

-- name: GetDailyCheckInStatsForWeek :many
SELECT
    ci.local_date AS day,
    COUNT(*) AS total_check_ins,
    COUNT(*) FILTER (WHERE ci.status = 'completed') AS completed_count,
    COUNT(*) FILTER (WHERE ci.status = 'missed') AS missed_count
FROM check_ins ci
WHERE ci.user_id = $1
  AND ci.local_date >= $2
  AND ci.local_date < $3
GROUP BY ci.local_date
ORDER BY day ASC;

-- name: GetBlockerStatsForWeek :many
SELECT blocker::text AS blocker, COUNT(*) AS count
FROM check_ins
WHERE user_id = $1
  AND local_date >= $2
  AND local_date < $3
  AND status = 'missed'
  AND blocker IS NOT NULL
GROUP BY blocker
ORDER BY count DESC;

-- name: GetMoodStatsForWeek :many
SELECT mood::text AS mood, COUNT(*) AS count
FROM check_ins
WHERE user_id = $1
  AND local_date >= $2
  AND local_date < $3
  AND mood IS NOT NULL
GROUP BY mood
ORDER BY count DESC;

-- name: GetEnergyStatsForWeek :many
SELECT energy::text AS energy, COUNT(*) AS count
FROM check_ins
WHERE user_id = $1
  AND local_date >= $2
  AND local_date < $3
  AND energy IS NOT NULL
GROUP BY energy
ORDER BY count DESC;
