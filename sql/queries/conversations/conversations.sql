-- name: CreateConversation :one
INSERT INTO conversations (user_id, title, type)
VALUES ($1, $2, $3)
RETURNING id, user_id, title, type, last_message, created_at, updated_at, archived;

-- name: GetConversation :one
SELECT id, user_id, title, type, last_message, created_at, updated_at, archived
FROM conversations
WHERE id = $1 AND user_id = $2;

-- name: ListConversations :many
-- The archived flag is a required parameter, not a tri-state: false (the zero
-- value, so also the default when a caller omits it) lists active
-- conversations and true lists archived ones. Archived conversations used to
-- leak into the default list because this query ignored the column entirely.
-- CountConversations MUST take the same filters or the pagination totals
-- describe a different result set than the page.
SELECT id, user_id, title, type, last_message, created_at, updated_at, archived
FROM conversations
WHERE user_id = $1
  AND archived = $5
  AND (type = $4 OR $4 = '')
ORDER BY updated_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: CountConversations :one
SELECT count(*) FROM conversations
WHERE user_id = $1
  AND archived = $3
  AND (type = $2 OR $2 = '');

-- name: UpdateConversationLastMessage :one
UPDATE conversations
SET last_message = $2, updated_at = now()
WHERE id = $1
RETURNING id, user_id, title, type, last_message, created_at, updated_at, archived;

-- name: UpdateConversationTitle :one
UPDATE conversations
SET title = $2
WHERE id = $1 AND user_id = $3
RETURNING id, user_id, title, type, last_message, created_at, updated_at, archived;

-- name: ArchiveConversation :one
UPDATE conversations
SET archived = true, updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING id, user_id, title, type, last_message, created_at, updated_at, archived;

-- name: UnarchiveConversation :one
UPDATE conversations
SET archived = false, updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING id, user_id, title, type, last_message, created_at, updated_at, archived;

-- name: DeleteConversation :exec
DELETE FROM conversations
WHERE id = $1 AND user_id = $2;
