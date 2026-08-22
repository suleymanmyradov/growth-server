-- name: ListNotifications :many
SELECT id, title, message, type, is_read, user_id, created_at, destination, resource_id, metadata FROM notifications
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListNotificationsForUser :many
SELECT id, title, message, type, is_read, user_id, created_at, destination, resource_id, metadata FROM notifications WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListUnreadNotifications :many
SELECT id, title, message, type, is_read, user_id, created_at, destination, resource_id, metadata FROM notifications WHERE user_id = $1 AND is_read = false
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListNotificationsByType :many
SELECT id, title, message, type, is_read, user_id, created_at, destination, resource_id, metadata FROM notifications WHERE user_id = $1 AND type = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: GetNotification :one
SELECT id, title, message, type, is_read, user_id, created_at, destination, resource_id, metadata FROM notifications WHERE id = $1;

-- name: CreateNotification :one
INSERT INTO notifications (title, message, type, user_id, destination, resource_id, deduplication_key, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (deduplication_key) WHERE deduplication_key IS NOT NULL DO UPDATE
SET deduplication_key = EXCLUDED.deduplication_key
RETURNING id, title, message, type, is_read, user_id, created_at, destination, resource_id, metadata;

-- name: MarkNotificationRead :one
UPDATE notifications
SET is_read = true
WHERE id = $1
RETURNING id, title, message, type, is_read, user_id, created_at, destination, resource_id, metadata;

-- name: MarkAllNotificationsRead :exec
UPDATE notifications
SET is_read = true
WHERE user_id = $1 AND is_read = false;

-- name: DeleteNotification :exec
DELETE FROM notifications WHERE id = $1;

-- name: DeleteAllNotificationsByUser :exec
DELETE FROM notifications WHERE user_id = $1;

-- name: ListNotificationsForUserKeyset :many
SELECT id, title, message, type, is_read, user_id, created_at, destination, resource_id, metadata FROM notifications
WHERE user_id = $1
  AND ($2::timestamptz IS NULL OR created_at < $2)
ORDER BY created_at DESC
LIMIT $3;

-- name: ListUnreadNotificationsKeyset :many
SELECT id, title, message, type, is_read, user_id, created_at, destination, resource_id, metadata FROM notifications
WHERE user_id = $1 AND is_read = false
  AND ($2::timestamptz IS NULL OR created_at < $2)
ORDER BY created_at DESC
LIMIT $3;

-- name: ListNotificationsByTypeKeyset :many
SELECT id, title, message, type, is_read, user_id, created_at, destination, resource_id, metadata FROM notifications
WHERE user_id = $1 AND type = $2
  AND ($3::timestamptz IS NULL OR created_at < $3)
ORDER BY created_at DESC
LIMIT $4;

-- name: CountNotificationsByUser :one
SELECT COUNT(*) FROM notifications WHERE user_id = $1;

-- name: GetUnreadCount :one
SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND is_read = false;

-- name: CreateNotificationsForUsers :execrows
INSERT INTO notifications (title, message, type, user_id, destination)
SELECT $1, $2, $3, user_id, 'notifications' FROM unnest($4::uuid[]) AS t(user_id);
