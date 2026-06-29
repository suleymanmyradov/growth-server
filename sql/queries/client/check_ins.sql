-- name: CreateCheckIn :one
-- Optimized: CTE fetches timezone once; direct VALUES insert instead of INSERT...SELECT.
WITH user_tz AS (
    SELECT COALESCE(timezone, 'UTC') AS tz
    FROM user_settings
    WHERE user_id = $1
)
INSERT INTO check_ins (user_id, habit_id, status, mood, energy, blocker, note, local_date)
VALUES ($1, $2, $3, $4, $5, $6, $7,
        (NOW() AT TIME ZONE COALESCE((SELECT tz FROM user_tz), 'UTC'))::date)
RETURNING id, user_id, habit_id, local_date, status, mood, energy, blocker, note, created_at;

-- name: GetTodayCheckIns :many
-- Optimized: CTE fetches timezone once; removed per-row LEFT JOIN.
WITH user_tz AS (
    SELECT COALESCE(timezone, 'UTC') AS tz
    FROM user_settings
    WHERE user_id = $1
)
SELECT ci.id, ci.user_id, ci.habit_id, ci.local_date, ci.status, ci.mood, ci.energy, ci.blocker, ci.note, ci.created_at
FROM check_ins ci
WHERE ci.user_id = $1
  AND ci.local_date = (NOW() AT TIME ZONE COALESCE((SELECT tz FROM user_tz), 'UTC'))::date;

-- name: GetCheckInsByHabit :many
SELECT id, user_id, habit_id, local_date, status, mood, energy, blocker, note, created_at
FROM check_ins
WHERE habit_id = $1 AND user_id = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: GetCheckInsByUser :many
SELECT id, user_id, habit_id, local_date, status, mood, energy, blocker, note, created_at
FROM check_ins
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetCheckInHistory :many
SELECT id, user_id, habit_id, local_date, status, mood, energy, blocker, note, created_at
FROM check_ins
WHERE user_id = $1
  AND created_at >= $2
  AND created_at < $3
ORDER BY created_at DESC
LIMIT $4 OFFSET $5;

-- name: GetCheckInsForWeek :many
SELECT id, user_id, habit_id, local_date, status, mood, energy, blocker, note, created_at
FROM check_ins
WHERE user_id = $1
  AND created_at >= sqlc.arg(week_start)
  AND created_at < sqlc.arg(week_end)
ORDER BY created_at DESC;

-- name: HasCheckedInToday :one
-- Optimized: CTE fetches timezone once; removed per-row LEFT JOIN.
WITH user_tz AS (
    SELECT COALESCE(timezone, 'UTC') AS tz
    FROM user_settings
    WHERE user_id = $1
)
SELECT EXISTS(
    SELECT 1 FROM check_ins ci
    WHERE ci.user_id = $1 AND ci.habit_id = $2
      AND ci.local_date = (NOW() AT TIME ZONE COALESCE((SELECT tz FROM user_tz), 'UTC'))::date
) AS exists;

-- name: GetCheckInsByUserKeyset :many
-- Keyset pagination: more efficient than OFFSET for deep pages.
SELECT id, user_id, habit_id, local_date, status, mood, energy, blocker, note, created_at
FROM check_ins
WHERE user_id = $1
  AND ($2::timestamptz IS NULL OR created_at < $2)
ORDER BY created_at DESC
LIMIT $3;

-- name: CountCheckInsByUser :one
SELECT COUNT(*) FROM check_ins
WHERE user_id = $1;

-- name: CountCheckInsByHabit :one
SELECT COUNT(*) FROM check_ins
WHERE habit_id = $1;
