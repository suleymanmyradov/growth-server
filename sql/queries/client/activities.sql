-- name: ListActivities :many
SELECT id, item_type, title, description, metadata, user_id, created_at FROM activities
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

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
    SUM(CASE WHEN item_type = 'article_saved' THEN 1 ELSE 0 END) AS article_saved
FROM activities
WHERE user_id = $1;

-- name: GetStreaks :one
WITH days AS (
    SELECT DISTINCT DATE(created_at) AS day
    FROM activities
    WHERE user_id = $1
), ordered_days AS (
    SELECT day, ROW_NUMBER() OVER (ORDER BY day) AS rn
    FROM days
), streak_groups AS (
    SELECT day, rn, day - (rn || ' days')::interval AS grp
    FROM ordered_days
), streak_lengths AS (
    SELECT MIN(day) AS start_day, MAX(day) AS end_day, COUNT(*) AS length
    FROM streak_groups
    GROUP BY grp
), current_streak AS (
    SELECT length
    FROM streak_lengths
    WHERE end_day = (SELECT MAX(day) FROM days)
    LIMIT 1
), longest_streak AS (
    SELECT MAX(length) AS length FROM streak_lengths
)
SELECT
    COALESCE((SELECT length FROM current_streak), 0) AS current_streak,
    COALESCE((SELECT length FROM longest_streak), 0) AS longest_streak;

-- name: GetAchievements :many
WITH user_activities AS (
    SELECT * FROM activities WHERE user_id = $1
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

-- name: GetActivityCalendar :many
SELECT DATE(created_at) AS day,
       COUNT(*) AS activity_count
FROM activities
WHERE user_id = $1
  AND created_at >= $2
  AND created_at < $3
GROUP BY 1
ORDER BY day;
