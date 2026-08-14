-- name: CreateCheckIn :one
-- Timezone is passed by the caller (fetched from UserPreferences interface).
INSERT INTO check_ins (user_id, habit_id, status, mood, energy, blocker, note, local_date)
VALUES ($1, $2, $3, $4, $5, $6, $7,
        (NOW() AT TIME ZONE sqlc.arg(timezone)::text)::date)
RETURNING id, user_id, habit_id, local_date, status, mood, energy, blocker, note, created_at;

-- name: UpsertCheckIn :one
-- Insert a check-in for today, or update the existing one if already present
-- (UNIQUE(habit_id, local_date) conflict). This lets users re-check-in to
-- change status (missed → completed) or add/update mood/energy/blocker/note.
-- created_at is preserved on update (the original check-in timestamp).
-- Timezone is passed by the caller.
INSERT INTO check_ins (user_id, habit_id, status, mood, energy, blocker, note, local_date)
VALUES ($1, $2, $3, $4, $5, $6, $7,
        (NOW() AT TIME ZONE sqlc.arg(timezone)::text)::date)
ON CONFLICT (habit_id, local_date) DO UPDATE SET
    status  = EXCLUDED.status,
    mood    = EXCLUDED.mood,
    energy  = EXCLUDED.energy,
    blocker = EXCLUDED.blocker,
    note    = EXCLUDED.note
RETURNING id, user_id, habit_id, local_date, status, mood, energy, blocker, note, created_at;

-- name: GetTodayCheckIns :many
-- Timezone is passed by the caller.
SELECT ci.id, ci.user_id, ci.habit_id, ci.local_date, ci.status, ci.mood, ci.energy, ci.blocker, ci.note, ci.created_at
FROM check_ins ci
WHERE ci.user_id = $1
  AND ci.local_date = (NOW() AT TIME ZONE sqlc.arg(timezone)::text)::date;

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
-- Timezone is passed by the caller.
SELECT EXISTS(
    SELECT 1 FROM check_ins ci
    WHERE ci.user_id = $1 AND ci.habit_id = $2
      AND ci.local_date = (NOW() AT TIME ZONE sqlc.arg(timezone)::text)::date
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

-- name: CountCompletedCheckInDays :one
-- Distinct local_date values with a completed check-in for any of the given
-- habits, within the [from_date, to_date] window (inclusive). Used by the
-- habit-driven goal progress formula (called via ICheckIns from goals logic).
SELECT COUNT(DISTINCT local_date) AS days
FROM check_ins
WHERE habit_id = ANY($1::uuid[])
  AND status = 'completed'
  AND local_date >= $2
  AND local_date <= $3;

-- name: DeleteTodayCheckIn :execrows
-- Deletes today's check-in for a specific habit (undo). The streak is derived
-- from check_ins history, so it recomputes automatically once today's
-- check-in is gone. Returns the number of rows deleted (0 = nothing to undo).
DELETE FROM check_ins
WHERE user_id = $1
  AND habit_id = $2
  AND local_date = (NOW() AT TIME ZONE sqlc.arg(timezone)::text)::date;
