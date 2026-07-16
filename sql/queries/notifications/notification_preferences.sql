-- name: GetNotificationPreferences :one
SELECT user_id, email_notifications, push_notifications, habit_reminders, goal_reminders, created_at, updated_at
FROM notification_preferences
WHERE user_id = $1;

-- name: UpsertNotificationPreferences :one
INSERT INTO notification_preferences (user_id, email_notifications, push_notifications, habit_reminders, goal_reminders)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id) DO UPDATE SET
    email_notifications = EXCLUDED.email_notifications,
    push_notifications = EXCLUDED.push_notifications,
    habit_reminders = EXCLUDED.habit_reminders,
    goal_reminders = EXCLUDED.goal_reminders
RETURNING user_id, email_notifications, push_notifications, habit_reminders, goal_reminders, created_at, updated_at;

-- name: DeleteNotificationPreferences :exec
DELETE FROM notification_preferences WHERE user_id = $1;
