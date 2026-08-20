-- name: UpsertHabitMissedStreak :one
-- Inserts or updates the missed-streak counter for a habit.
-- On a 'completed' check-in, the caller passes consecutive_missed_days=0
-- and last_completed_date=today to reset the streak.
INSERT INTO habit_missed_streaks (habit_id, user_id, consecutive_missed_days, last_completed_date, last_recovery_triggered)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (habit_id) DO UPDATE SET
    consecutive_missed_days = EXCLUDED.consecutive_missed_days,
    last_completed_date = EXCLUDED.last_completed_date,
    last_recovery_triggered = EXCLUDED.last_recovery_triggered
RETURNING habit_id, user_id, consecutive_missed_days, last_completed_date, last_recovery_triggered, updated_at;

-- name: GetHabitMissedStreak :one
SELECT habit_id, user_id, consecutive_missed_days, last_completed_date, last_recovery_triggered, updated_at
FROM habit_missed_streaks
WHERE habit_id = $1;

-- name: DeleteHabitMissedStreak :exec
DELETE FROM habit_missed_streaks WHERE habit_id = $1;

-- name: IncrementHabitMissedStreak :one
-- Atomically increments the missed counter and returns the new value.
-- If no row exists yet, inserts one with consecutive_missed_days=1.
INSERT INTO habit_missed_streaks (habit_id, user_id, consecutive_missed_days, last_completed_date)
VALUES ($1, $2, 1, NULL)
ON CONFLICT (habit_id) DO UPDATE SET
    consecutive_missed_days = habit_missed_streaks.consecutive_missed_days + 1
RETURNING habit_id, user_id, consecutive_missed_days, last_completed_date, last_recovery_triggered, updated_at;

-- name: ResetHabitMissedStreak :one
-- Resets the missed counter to 0 and sets last_completed_date.
-- Used when a 'completed' check-in arrives.
UPDATE habit_missed_streaks
SET consecutive_missed_days = 0,
    last_completed_date = $3,
    last_recovery_triggered = NULL
WHERE habit_id = $1 AND user_id = $2
RETURNING habit_id, user_id, consecutive_missed_days, last_completed_date, last_recovery_triggered, updated_at;
