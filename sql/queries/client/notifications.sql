-- name: ListNotifications :many
SELECT id, title, message, item_type, read, user_id, created_at FROM notifications
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListNotificationsByUser :many
SELECT id, title, message, item_type, read, user_id, created_at FROM notifications WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListUnreadNotifications :many
SELECT id, title, message, item_type, read, user_id, created_at FROM notifications WHERE user_id = $1 AND read = false
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListNotificationsByType :many
SELECT id, title, message, item_type, read, user_id, created_at FROM notifications WHERE user_id = $1 AND item_type = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: GetNotification :one
SELECT id, title, message, item_type, read, user_id, created_at FROM notifications WHERE id = $1;

-- name: CreateNotification :one
INSERT INTO notifications (title, message, item_type, user_id)
VALUES ($1, $2, $3, $4)
RETURNING id, title, message, item_type, read, user_id, created_at;

-- name: MarkNotificationRead :one
UPDATE notifications
SET read = true
WHERE id = $1
RETURNING id, title, message, item_type, read, user_id, created_at;

-- name: MarkAllNotificationsRead :exec
UPDATE notifications
SET read = true
WHERE user_id = $1 AND read = false;

-- name: DeleteNotification :exec
DELETE FROM notifications WHERE id = $1;

-- name: DeleteAllNotificationsByUser :exec
DELETE FROM notifications WHERE user_id = $1;

-- name: CountNotifications :one
SELECT COUNT(*) FROM notifications;

-- name: CountNotificationsByUser :one
SELECT COUNT(*) FROM notifications WHERE user_id = $1;

-- name: CountUnreadNotifications :one
SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND read = false;

-- name: ListNotificationsForUser :many
SELECT id, title, message, item_type, read, user_id, created_at
FROM notifications
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetUnreadCount :one
SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND read = false;
