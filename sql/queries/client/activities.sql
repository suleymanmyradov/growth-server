-- name: ListActivities :many
-- NOTE: Previously unfiltered; now requires user_id to avoid full table scans on a 50GB table.
SELECT id, item_type, title, description, metadata, user_id, created_at FROM activities
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListActivitiesByType :many
SELECT id, item_type, title, description, metadata, user_id, created_at FROM activities WHERE user_id = $1 AND item_type = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: GetActivity :one
SELECT id, item_type, title, description, metadata, user_id, created_at FROM activities WHERE id = $1;

-- name: CreateActivity :one
INSERT INTO activities (item_type, title, description, metadata, user_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, item_type, title, description, metadata, user_id, created_at;

-- name: DeleteActivity :exec
DELETE FROM activities WHERE id = $1;

-- name: DeleteActivitiesByUser :exec
DELETE FROM activities WHERE user_id = $1;

-- name: CountActivitiesByUser :one
SELECT COUNT(*) FROM activities WHERE user_id = $1;

-- name: CountActivitiesByUserAndType :one
SELECT COUNT(*) FROM activities WHERE user_id = $1 AND item_type = $2;

-- name: GetActivityFeed :many
SELECT id, item_type, title, description, metadata, user_id, created_at
FROM activities
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: LogActivity :one
INSERT INTO activities (item_type, title, description, metadata, user_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, item_type, title, description, metadata, user_id, created_at;

-- name: GetActivityStats :one
SELECT
    COUNT(*) AS total_activities,
    COUNT(*) FILTER (WHERE item_type = 'habit_completed') AS habit_completed,
    COUNT(*) FILTER (WHERE item_type = 'goal_created') AS goal_created,
    COUNT(*) FILTER (WHERE item_type = 'goal_completed') AS goal_completed,
    COUNT(*) FILTER (WHERE item_type = 'article_saved') AS article_saved,
    COUNT(*) FILTER (WHERE item_type = 'check_in_completed') AS check_in_completed,
    COUNT(*) FILTER (WHERE item_type = 'check_in_missed') AS check_in_missed
FROM activities
WHERE user_id = $1;

-- name: GetStreaks :one
-- Optimized: uses check_ins.local_date (indexed) instead of DATE(created_at) on activities.
-- Simplified CTEs: removed string concatenation + interval cast; uses date - integer arithmetic.
WITH days AS (
    SELECT DISTINCT local_date AS day
    FROM check_ins
    WHERE user_id = $1
      AND status = 'completed'
), numbered AS (
    SELECT day, ROW_NUMBER() OVER (ORDER BY day) AS rn
    FROM days
), groups AS (
    SELECT day, day - rn::int AS grp
    FROM numbered
), streaks AS (
    SELECT grp, COUNT(*) AS length, MAX(day) AS max_day
    FROM groups
    GROUP BY grp
)
SELECT
    COALESCE(
        (SELECT length FROM streaks WHERE max_day = (SELECT MAX(day) FROM days) LIMIT 1),
        0
    ) AS current_streak,
    COALESCE((SELECT MAX(length) FROM streaks), 0) AS longest_streak;

-- name: GetAchievements :many
-- Single aggregate pass over activities instead of repeated subqueries.
WITH s AS (
  SELECT
    MIN(created_at)                                              AS first_at,
    COUNT(*) FILTER (WHERE item_type = 'habit_completed')          AS habit_done,
    MAX(created_at) FILTER (WHERE item_type = 'habit_completed')   AS last_habit_at,
    COUNT(*) FILTER (WHERE item_type = 'goal_completed')           AS goal_done,
    MAX(created_at) FILTER (WHERE item_type = 'goal_completed')    AS last_goal_at
  FROM activities WHERE user_id = $1
)
SELECT 'first_activity'::text AS id,
       'First Activity'::text AS name,
       'Complete your first activity'::text AS description,
       NULL::text AS icon_url,
       s.first_at AS unlocked_at
FROM s
UNION ALL
SELECT 'habit_10', 'Habit Enthusiast', 'Complete 10 habits', NULL,
       CASE WHEN s.habit_done >= 10 THEN s.last_habit_at ELSE NULL END
FROM s
UNION ALL
SELECT 'goal_5', 'Goal Crusher', 'Complete 5 goals', NULL,
       CASE WHEN s.goal_done >= 5 THEN s.last_goal_at ELSE NULL END
FROM s
ORDER BY unlocked_at NULLS LAST;

-- name: ListActivitiesByTypes :many
-- Filter activities by multiple types in a single round-trip.
SELECT id, item_type, title, description, metadata, user_id, created_at
FROM activities
WHERE user_id = $1
  AND item_type = ANY($2::activity_type[])
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: GetActivityCalendar :many
SELECT DATE(created_at) AS day,
       COUNT(*) AS activity_count
FROM activities
WHERE user_id = $1
  AND created_at >= $2
  AND created_at < $3
GROUP BY 1
ORDER BY day;

-- name: ListActivitiesKeyset :many
-- Keyset pagination: more efficient than OFFSET for deep pages.
-- Pass last_created_at from previous page (or NULL for first page).
SELECT id, item_type, title, description, metadata, user_id, created_at
FROM activities
WHERE user_id = $1
  AND ($2::timestamptz IS NULL OR created_at < $2)
ORDER BY created_at DESC
LIMIT $3;
