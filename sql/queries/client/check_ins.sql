-- name: CreateCheckIn :one
INSERT INTO check_ins (user_id, habit_id, status, mood, energy, blocker, note)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetTodayCheckIns :many
SELECT * FROM check_ins
WHERE user_id = $1
  AND created_at::date = CURRENT_DATE;

-- name: GetCheckInsByHabit :many
SELECT * FROM check_ins
WHERE habit_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetCheckInsByUser :many
SELECT * FROM check_ins
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetCheckInsForWeek :many
SELECT * FROM check_ins
WHERE user_id = $1
  AND created_at >= $2
  AND created_at < $3
ORDER BY created_at DESC;

-- name: HasCheckedInToday :one
SELECT EXISTS(
    SELECT 1 FROM check_ins
    WHERE user_id = $1 AND habit_id = $2
      AND created_at::date = CURRENT_DATE
) AS exists;

-- name: CountCheckInsByUser :one
SELECT COUNT(*) FROM check_ins
WHERE user_id = $1;

-- name: CountCheckInsByHabit :one
SELECT COUNT(*) FROM check_ins
WHERE habit_id = $1;
