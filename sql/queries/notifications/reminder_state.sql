-- name: GetReminderState :one
SELECT user_id, timezone, check_in_time, habit_reminders, onboarding_completed,
       active_habit_count, last_check_in_date, checked_in_count_today, updated_at
FROM reminder_state WHERE user_id = $1;

-- name: UpsertReminderStateSettings :exec
INSERT INTO reminder_state (user_id, timezone, check_in_time, habit_reminders)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id) DO UPDATE SET
    timezone = EXCLUDED.timezone,
    check_in_time = EXCLUDED.check_in_time,
    habit_reminders = EXCLUDED.habit_reminders;

-- name: SetOnboardingCompleted :exec
INSERT INTO reminder_state (user_id, onboarding_completed)
VALUES ($1, true)
ON CONFLICT (user_id) DO UPDATE SET onboarding_completed = true;

-- name: IncrementHabitCount :exec
INSERT INTO reminder_state (user_id, active_habit_count)
VALUES ($1, 1)
ON CONFLICT (user_id) DO UPDATE SET active_habit_count = reminder_state.active_habit_count + 1;

-- name: DecrementHabitCount :exec
UPDATE reminder_state SET active_habit_count = GREATEST(active_habit_count - 1, 0)
WHERE user_id = $1;

-- name: BumpCheckInCountToday :exec
INSERT INTO reminder_state (user_id, checked_in_count_today, last_check_in_date)
VALUES ($1, 1, CURRENT_DATE)
ON CONFLICT (user_id) DO UPDATE SET
    checked_in_count_today = CASE
        WHEN reminder_state.last_check_in_date = CURRENT_DATE THEN reminder_state.checked_in_count_today + 1
        ELSE 1
    END,
    last_check_in_date = CURRENT_DATE;

-- name: DeleteReminderState :exec
DELETE FROM reminder_state WHERE user_id = $1;
