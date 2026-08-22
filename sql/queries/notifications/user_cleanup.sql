-- Bulk cleanup queries for user_deleted event consumers.

-- name: DeleteNotificationsByUser :exec
DELETE FROM notifications WHERE user_id = $1;

-- name: DeleteRemindersByUser :exec
DELETE FROM reminders WHERE user_id = $1;

-- name: DeleteNotificationPreferencesByUser :exec
DELETE FROM notification_preferences WHERE user_id = $1;

-- name: DeleteDevicesByUser :exec
DELETE FROM notification_devices WHERE user_id = $1;

-- name: DeleteNotificationRecipientByUser :exec
DELETE FROM notification_recipients WHERE user_id = $1;

-- name: DeleteNotificationHabitStatesByUserCleanup :exec
DELETE FROM notification_habit_state WHERE user_id = $1;
