-- name: GetReminderContext :one
SELECT
    us.timezone,
    us.check_in_time,
    COALESCE(us.habit_reminders, FALSE) AS habit_reminders,
    us.onboarding_completed,
    (SELECT COUNT(*) FROM habits h WHERE h.user_id = $1) AS active_habit_count,
    EXISTS(
        SELECT 1 FROM check_ins ci
        WHERE ci.user_id = $1
          AND ci.created_at::date = (NOW() AT TIME ZONE us.timezone)::date
    ) AS checked_in_today
FROM user_settings us
WHERE us.user_id = $1;

-- name: MarkReminderSent :one
UPDATE reminder_queue
SET sent = TRUE, sent_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;
