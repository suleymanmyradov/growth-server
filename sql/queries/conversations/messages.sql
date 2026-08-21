-- name: CreateMessage :one
INSERT INTO conversation_messages (conversation_id, role, content)
VALUES ($1, $2, $3)
RETURNING id, conversation_id, role, content, created_at;

-- name: ListMessages :many
WITH cte AS (
  SELECT id, conversation_id, role, content, created_at
  FROM conversation_messages
  WHERE conversation_id = $1
  ORDER BY created_at DESC
  LIMIT $2 OFFSET $3
)
SELECT id, conversation_id, role, content, created_at
FROM cte
ORDER BY created_at ASC;

-- name: CountMessages :one
SELECT count(*) FROM conversation_messages
WHERE conversation_id = $1;

-- name: GetLastMessage :one
SELECT id, conversation_id, role, content, created_at
FROM conversation_messages
WHERE conversation_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: RegenerateLastResponse :one
DELETE FROM conversation_messages AS target
WHERE target.id = (
  SELECT message.id
  FROM conversation_messages AS message
  WHERE message.conversation_id = $1
  ORDER BY message.created_at DESC
  LIMIT 1
)
  AND target.role = 'assistant'
RETURNING target.id, target.conversation_id, target.role, target.content, target.created_at;
