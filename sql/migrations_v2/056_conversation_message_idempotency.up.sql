-- Idempotency for conversation message appends.
--
-- Without this, a client retry (flaky network, double-tap on send, an
-- automatic retry in the BFF) creates a second copy of the same user turn.
-- The streaming path now aborts when a persistence write fails, which makes
-- retries the expected recovery path rather than an edge case -- so the write
-- has to be safe to repeat.
--
-- client_message_id is supplied by the client, unique per (conversation,
-- client id), and nullable so server-authored messages (assistant replies,
-- crisis responses) are unaffected. A partial index keeps the uniqueness
-- constraint off the NULL rows.

ALTER TABLE conversation_messages
    ADD COLUMN client_message_id text;

CREATE UNIQUE INDEX idx_conversation_messages_client_id
    ON conversation_messages (conversation_id, client_message_id)
    WHERE client_message_id IS NOT NULL;
