-- name: CreateMessage :one
INSERT INTO conversation_messages (conversation_id, role, content)
VALUES ($1, $2, $3)
RETURNING id, conversation_id, role, content, created_at, client_message_id;

-- name: CreateMessageIdempotent :one
-- Append a client-authored message at most once per (conversation,
-- client_message_id).
--
-- DO UPDATE with a self-assignment rather than DO NOTHING: DO NOTHING makes
-- RETURNING yield no row on conflict, which would force a second read -- and
-- two concurrent retries (a double-tapped send) could each miss the other's
-- uncommitted row and both come back empty. DO UPDATE returns the existing row
-- and takes a row lock, so concurrent retries serialize and both callers get
-- the same message back.
INSERT INTO conversation_messages (conversation_id, role, content, client_message_id)
VALUES ($1, $2, $3, $4)
ON CONFLICT (conversation_id, client_message_id) WHERE client_message_id IS NOT NULL
DO UPDATE SET content = conversation_messages.content
RETURNING id, conversation_id, role, content, created_at, client_message_id;

-- name: ListMessages :many
-- Returns the newest page of messages, re-sorted oldest-to-newest for display.
-- Ordering is (created_at, id), never created_at alone: two messages written in
-- the same millisecond (a user turn and its assistant reply) would otherwise
-- reorder between reads and render as a reply before its question. id is a
-- uuid_generate_v7() value, so it is itself time-ordered and the tiebreak is
-- correct by construction rather than arbitrary.
WITH cte AS (
  SELECT id, conversation_id, role, content, created_at, client_message_id
  FROM conversation_messages
  WHERE conversation_id = $1
  ORDER BY created_at DESC, id DESC
  LIMIT $2 OFFSET $3
)
SELECT id, conversation_id, role, content, created_at, client_message_id
FROM cte
ORDER BY created_at ASC, id ASC;

-- name: CountMessages :one
SELECT count(*) FROM conversation_messages
WHERE conversation_id = $1;

-- name: GetLastMessage :one
SELECT id, conversation_id, role, content, created_at, client_message_id
FROM conversation_messages
WHERE conversation_id = $1
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: RegenerateLastResponse :one
DELETE FROM conversation_messages AS target
WHERE target.id = (
  SELECT message.id
  FROM conversation_messages AS message
  WHERE message.conversation_id = $1
  ORDER BY message.created_at DESC, message.id DESC
  LIMIT 1
)
  AND target.role = 'assistant'
RETURNING target.id, target.conversation_id, target.role, target.content, target.created_at, target.client_message_id;
