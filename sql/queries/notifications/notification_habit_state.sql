-- name: UpsertNotificationHabitState :one
INSERT INTO notification_habit_state (
    user_id, habit_id, habit_name, current_streak, last_check_in_local_date, active
)
VALUES ($1, $2, $3, $4, $5, true)
ON CONFLICT (user_id, habit_id) DO UPDATE SET
    habit_name = EXCLUDED.habit_name,
    current_streak = EXCLUDED.current_streak,
    last_check_in_local_date = EXCLUDED.last_check_in_local_date,
    active = true,
    updated_at = now()
RETURNING *;

-- name: CreateNotificationHabitState :one
INSERT INTO notification_habit_state (user_id, habit_id, habit_name)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, habit_id) DO UPDATE SET
    habit_name = EXCLUDED.habit_name,
    active = true,
    updated_at = now()
RETURNING *;

-- name: DeleteNotificationHabitState :exec
DELETE FROM notification_habit_state WHERE user_id = $1 AND habit_id = $2;

-- name: ListHabitsAtStreakRisk :many
SELECT *
FROM notification_habit_state
WHERE user_id = $1
  AND active = true
  AND current_streak > 0
  AND (last_check_in_local_date IS NULL OR last_check_in_local_date < $2::date)
ORDER BY current_streak DESC, habit_name
LIMIT $3;

-- name: DeleteNotificationHabitStatesByUser :exec
DELETE FROM notification_habit_state WHERE user_id = $1;
