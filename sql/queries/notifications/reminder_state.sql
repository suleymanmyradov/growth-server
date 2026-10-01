-- name: GetReminderState :one
SELECT user_id, timezone, check_in_time, habit_reminders, onboarding_completed,
       active_habit_count, last_check_in_date, checked_in_count_today, updated_at
FROM reminder_state WHERE user_id = $1;

-- name: UpsertReminderStateSettings :exec
-- SettingsChanged events may carry only the fields that changed. An empty
-- timezone or a NULL check_in_time means "not provided" and must preserve the
-- existing value rather than overwriting it (a partial PUT /settings would
-- otherwise reset timezone to UTC or check_in_time to NULL — which is also
-- impossible since the column is NOT NULL).
INSERT INTO reminder_state (user_id, timezone, check_in_time, habit_reminders)
VALUES ($1, COALESCE(NULLIF($2::varchar, ''), 'UTC'), COALESCE($3::time, '09:00'::time), $4)
ON CONFLICT (user_id) DO UPDATE SET
    timezone = COALESCE(NULLIF(EXCLUDED.timezone, ''), reminder_state.timezone),
    check_in_time = COALESCE(EXCLUDED.check_in_time, reminder_state.check_in_time),
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
-- $2 is the check-in's local date (YYYY-MM-DD in the owner's timezone, from
-- check_ins.local_date via the check_in_created event — or the consumer's
-- local-date derivation for legacy events). Comparing against CURRENT_DATE
-- here would be the UTC date and fire false "missed" pushes for users whose
-- local day differs from the UTC day.
INSERT INTO reminder_state (user_id, checked_in_count_today, last_check_in_date)
VALUES ($1, 1, $2::date)
ON CONFLICT (user_id) DO UPDATE SET
    checked_in_count_today = CASE
        WHEN reminder_state.last_check_in_date = $2::date THEN reminder_state.checked_in_count_today + 1
        ELSE 1
    END,
    last_check_in_date = $2::date;

-- name: DeleteReminderState :exec
DELETE FROM reminder_state WHERE user_id = $1;
