-- name: ListNotifications :many
SELECT id, title, message, item_type, is_read, user_id, created_at FROM notifications
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListNotificationsForUser :many
SELECT id, title, message, item_type, is_read, user_id, created_at FROM notifications WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListUnreadNotifications :many
SELECT id, title, message, item_type, is_read, user_id, created_at FROM notifications WHERE user_id = $1 AND is_read = false
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListNotificationsByType :many
SELECT id, title, message, item_type, is_read, user_id, created_at FROM notifications WHERE user_id = $1 AND item_type = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: GetNotification :one
SELECT id, title, message, item_type, is_read, user_id, created_at FROM notifications WHERE id = $1;

-- name: CreateNotification :one
INSERT INTO notifications (title, message, item_type, user_id)
VALUES ($1, $2, $3, $4)
RETURNING id, title, message, item_type, is_read, user_id, created_at;

-- name: MarkNotificationRead :one
UPDATE notifications
SET is_read = true
WHERE id = $1
RETURNING id, title, message, item_type, is_read, user_id, created_at;

-- name: MarkAllNotificationsRead :exec
UPDATE notifications
SET is_read = true
WHERE user_id = $1 AND is_read = false;

-- name: DeleteNotification :exec
DELETE FROM notifications WHERE id = $1;

-- name: DeleteAllNotificationsByUser :exec
DELETE FROM notifications WHERE user_id = $1;

-- name: ListNotificationsForUserKeyset :many
-- Keyset pagination: more efficient than OFFSET for deep pages.
SELECT id, title, message, item_type, is_read, user_id, created_at FROM notifications
WHERE user_id = $1
  AND ($2::timestamptz IS NULL OR created_at < $2)
ORDER BY created_at DESC
LIMIT $3;

-- name: ListUnreadNotificationsKeyset :many
-- Keyset pagination for unread notifications feed.
SELECT id, title, message, item_type, is_read, user_id, created_at FROM notifications
WHERE user_id = $1 AND is_read = false
  AND ($2::timestamptz IS NULL OR created_at < $2)
ORDER BY created_at DESC
LIMIT $3;

-- name: ListNotificationsByTypeKeyset :many
-- Keyset pagination for typed notification feeds.
SELECT id, title, message, item_type, is_read, user_id, created_at FROM notifications
WHERE user_id = $1 AND item_type = $2
  AND ($3::timestamptz IS NULL OR created_at < $3)
ORDER BY created_at DESC
LIMIT $4;

-- name: CountNotificationsByUser :one
SELECT COUNT(*) FROM notifications WHERE user_id = $1;

-- name: GetUnreadCount :one
SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND is_read = false;

