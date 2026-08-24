-- Deterministic ordering for conversation history.
--
-- ListMessages pages the NEWEST messages (ORDER BY created_at DESC, id DESC)
-- and re-sorts them ascending for display. Ordering on created_at alone let two
-- messages written in the same millisecond -- a user turn and its assistant
-- reply -- swap between reads, rendering a reply before its question.
--
-- id is a uuid_generate_v7() value and therefore time-ordered, so (created_at,
-- id) is a correct tiebreak rather than an arbitrary one.
--
-- The existing idx_conversation_messages_conversation_id (conversation_id,
-- created_at) cannot serve the DESC page without a sort step, so this index
-- matches the query's actual direction.

CREATE INDEX IF NOT EXISTS idx_conversation_messages_recent
    ON conversation_messages (conversation_id, created_at DESC, id DESC);
