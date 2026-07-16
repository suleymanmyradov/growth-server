-- Bulk cleanup queries for user_deleted event consumers.

-- name: DeleteNotificationsByUser :exec
DELETE FROM notifications WHERE user_id = $1;

-- name: DeleteRemindersByUser :exec
DELETE FROM reminders WHERE user_id = $1;

-- name: DeleteNotificationPreferencesByUser :exec
DELETE FROM notification_preferences WHERE user_id = $1;
