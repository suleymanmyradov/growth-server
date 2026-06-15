-- name: GetReminderContext :one
-- Optimized: single scan of habits + one lookup of user_settings.
-- Replaces correlated NOT EXISTS per-habit with a LEFT JOIN aggregate.
WITH user_settings_row AS (
    SELECT COALESCE(timezone, 'UTC') AS tz,
           check_in_time,
           COALESCE(habit_reminders, FALSE) AS habit_reminders,
           COALESCE(onboarding_completed, FALSE) AS onboarding_completed
    FROM user_settings
    WHERE user_id = $1
)
SELECT
    COALESCE(usr.tz, 'UTC') AS timezone,
    usr.check_in_time,
    COALESCE(usr.habit_reminders, FALSE) AS habit_reminders,
    COALESCE(usr.onboarding_completed, FALSE) AS onboarding_completed,
    COUNT(h.id) AS active_habit_count,
    (COUNT(h.id) = COUNT(ci.id)) AS checked_in_today
FROM (SELECT $1::uuid AS uid) dummy
LEFT JOIN user_settings_row usr ON true
LEFT JOIN habits h ON h.user_id = dummy.uid
LEFT JOIN check_ins ci ON ci.habit_id = h.id
    AND ci.local_date = (NOW() AT TIME ZONE COALESCE(usr.tz, 'UTC'))::date
GROUP BY usr.tz, usr.check_in_time, usr.habit_reminders, usr.onboarding_completed;

-- name: MarkReminderSent :one
UPDATE reminders
SET sent_at = now()
WHERE id = $1
RETURNING *;
