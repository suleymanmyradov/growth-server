-- name: CreateConversation :one
INSERT INTO conversations (user_id, title, type)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetConversation :one
SELECT * FROM conversations
WHERE id = $1 AND user_id = $2;

-- name: ListConversations :many
SELECT * FROM conversations
WHERE user_id = $1 AND archived = false
ORDER BY updated_at DESC
LIMIT $2 OFFSET $3;

-- name: ListArchivedConversations :many
SELECT * FROM conversations
WHERE user_id = $1 AND archived = true
ORDER BY updated_at DESC
LIMIT $2 OFFSET $3;

-- name: CountConversations :one
SELECT count(*) FROM conversations
WHERE user_id = $1 AND archived = false;

-- name: CountArchivedConversations :one
SELECT count(*) FROM conversations
WHERE user_id = $1 AND archived = true;

-- name: UpdateConversationLastMessage :one
UPDATE conversations
SET last_message = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateConversationTitle :one
UPDATE conversations
SET title = $2
WHERE id = $1 AND user_id = $3
RETURNING *;

-- name: ArchiveConversation :one
UPDATE conversations
SET archived = true, updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: UnarchiveConversation :one
UPDATE conversations
SET archived = false, updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: DeleteConversation :exec
DELETE FROM conversations
WHERE id = $1 AND user_id = $2;
