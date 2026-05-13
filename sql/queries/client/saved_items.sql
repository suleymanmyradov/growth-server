-- name: ListSavedItems :many
SELECT id, item_type, item_id, user_id, created_at FROM saved_items
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListSavedItemsByUser :many
SELECT id, item_type, item_id, user_id, created_at FROM saved_items WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListSavedItemsByType :many
SELECT id, item_type, item_id, user_id, created_at FROM saved_items WHERE user_id = $1 AND item_type = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: GetSavedItem :one
SELECT id, item_type, item_id, user_id, created_at FROM saved_items WHERE id = $1;

-- name: GetSavedItemByUserAndItem :one
SELECT id, item_type, item_id, user_id, created_at FROM saved_items WHERE user_id = $1 AND item_type = $2 AND item_id = $3;

-- name: CreateSavedItem :one
INSERT INTO saved_items (item_type, item_id, user_id)
VALUES ($1, $2, $3)
RETURNING id, item_type, item_id, user_id, created_at;

-- name: DeleteSavedItem :exec
DELETE FROM saved_items WHERE id = $1;

-- name: DeleteSavedItemByUserAndItem :exec
DELETE FROM saved_items WHERE user_id = $1 AND item_type = $2 AND item_id = $3;

-- name: IsItemSaved :one
SELECT EXISTS(SELECT 1 FROM saved_items WHERE user_id = $1 AND item_type = $2 AND item_id = $3);

-- name: CountSavedItems :one
SELECT COUNT(*) FROM saved_items;

-- name: CountSavedItemsByUser :one
SELECT COUNT(*) FROM saved_items WHERE user_id = $1;

-- name: CountSavedItemsByUserAndType :one
SELECT COUNT(*) FROM saved_items WHERE user_id = $1 AND item_type = $2;
