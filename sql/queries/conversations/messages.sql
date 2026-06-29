-- name: CreateMessage :one
INSERT INTO conversation_messages (conversation_id, role, content)
VALUES ($1, $2, $3)
RETURNING id, conversation_id, role, content, created_at;

-- name: ListMessages :many
SELECT id, conversation_id, role, content, created_at
FROM conversation_messages
WHERE conversation_id = $1
ORDER BY created_at ASC
LIMIT $2 OFFSET $3;

-- name: CountMessages :one
SELECT count(*) FROM conversation_messages
WHERE conversation_id = $1;

-- name: GetLastMessage :one
SELECT id, conversation_id, role, content, created_at
FROM conversation_messages
WHERE conversation_id = $1
ORDER BY created_at DESC
LIMIT 1;
