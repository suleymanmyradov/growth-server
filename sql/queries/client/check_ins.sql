-- name: CreateCheckIn :one
INSERT INTO check_ins (user_id, habit_id, status, mood, energy, blocker, note, local_date)
SELECT $1, $2, $3, $4, $5, $6, $7, (NOW() AT TIME ZONE COALESCE(
  (SELECT timezone FROM user_settings WHERE user_id = $1),
  'UTC'
))::date
RETURNING *;

-- name: GetTodayCheckIns :many
SELECT ci.* FROM check_ins ci
LEFT JOIN user_settings us ON ci.user_id = us.user_id
WHERE ci.user_id = $1
  AND ci.local_date = (NOW() AT TIME ZONE COALESCE(us.timezone, 'UTC'))::date;

-- name: GetCheckInsByHabit :many
SELECT * FROM check_ins
WHERE habit_id = $1 AND user_id = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: GetCheckInsByUser :many
SELECT * FROM check_ins
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetCheckInHistory :many
SELECT * FROM check_ins
WHERE user_id = $1
  AND created_at >= $2
  AND created_at < $3
ORDER BY created_at DESC
LIMIT $4 OFFSET $5;

-- name: GetCheckInsForWeek :many
SELECT * FROM check_ins
WHERE user_id = $1
  AND created_at >= sqlc.arg(week_start)
  AND created_at < sqlc.arg(week_end)
ORDER BY created_at DESC;

-- name: HasCheckedInToday :one
SELECT EXISTS(
    SELECT 1 FROM check_ins ci
    LEFT JOIN user_settings us ON ci.user_id = us.user_id
    WHERE ci.user_id = $1 AND ci.habit_id = $2
      AND ci.local_date = (NOW() AT TIME ZONE COALESCE(us.timezone, 'UTC'))::date
) AS exists;

-- name: CountCheckInsByUser :one
SELECT COUNT(*) FROM check_ins
WHERE user_id = $1;

-- name: CountCheckInsByHabit :one
SELECT COUNT(*) FROM check_ins
WHERE habit_id = $1;
