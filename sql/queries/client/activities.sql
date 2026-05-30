-- name: ListActivities :many
-- NOTE: Previously unfiltered; now requires user_id to avoid full table scans on a 50GB table.
SELECT id, item_type, title, description, metadata, user_id, created_at FROM activities
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListActivitiesByUser :many
SELECT id, item_type, title, description, metadata, user_id, created_at FROM activities WHERE user_id = $1
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

-- name: CountActivities :one
SELECT COUNT(*) FROM activities;

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
    SUM(CASE WHEN item_type = 'habit_completed' THEN 1 ELSE 0 END) AS habit_completed,
    SUM(CASE WHEN item_type = 'goal_created' THEN 1 ELSE 0 END) AS goal_created,
    SUM(CASE WHEN item_type = 'goal_completed' THEN 1 ELSE 0 END) AS goal_completed,
    SUM(CASE WHEN item_type = 'article_saved' THEN 1 ELSE 0 END) AS article_saved,
    SUM(CASE WHEN item_type = 'check_in_completed' THEN 1 ELSE 0 END) AS check_in_completed,
    SUM(CASE WHEN item_type = 'check_in_missed' THEN 1 ELSE 0 END) AS check_in_missed
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
WITH user_activities AS (
    SELECT id, item_type, title, description, metadata, user_id, created_at FROM activities WHERE user_id = $1
), achievement_rows AS (
    SELECT 'first_activity'::text AS id,
           'First Activity'::text AS name,
           'Complete your first activity'::text AS description,
           NULL::text AS icon_url,
           MIN(created_at) AS unlocked_at
    FROM user_activities
    UNION ALL
    SELECT 'habit_10', 'Habit Enthusiast', 'Complete 10 habits', NULL,
           MAX(created_at)
    FROM user_activities WHERE item_type = 'habit_completed'
    GROUP BY user_id HAVING COUNT(*) >= 10
    UNION ALL
    SELECT 'goal_5', 'Goal Crusher', 'Complete 5 goals', NULL,
           MAX(created_at)
    FROM user_activities WHERE item_type = 'goal_completed'
    GROUP BY user_id HAVING COUNT(*) >= 5
)
SELECT id, name, description, icon_url, unlocked_at
FROM achievement_rows
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
