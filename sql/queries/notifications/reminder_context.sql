-- name: GetReminderContext :one
SELECT
    COALESCE(us.timezone, 'UTC') AS timezone,
    us.check_in_time,
    COALESCE(us.habit_reminders, FALSE) AS habit_reminders,
    COALESCE(us.onboarding_completed, FALSE) AS onboarding_completed,
    (SELECT COUNT(*) FROM habits h WHERE h.user_id = $1) AS active_habit_count,
    -- checked_in_today is TRUE only if the user has checked in on ALL active habits today
    -- This prevents reminder spam while still sending reminders for incomplete habits
    (SELECT COUNT(*) = 0
     FROM habits h
     WHERE h.user_id = $1
       AND NOT EXISTS (
           SELECT 1 FROM check_ins ci
           WHERE ci.habit_id = h.id
             AND ci.local_date = (NOW() AT TIME ZONE COALESCE(us.timezone, 'UTC'))::date
       )
    ) AS checked_in_today
FROM (SELECT $1 AS user_id) dummy
LEFT JOIN user_settings us ON us.user_id = dummy.user_id;

-- name: MarkReminderSent :one
UPDATE reminder_queue
SET sent = TRUE, sent_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;
