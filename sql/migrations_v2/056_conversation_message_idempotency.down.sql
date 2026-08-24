DROP INDEX IF EXISTS idx_conversation_messages_client_id;
ALTER TABLE conversation_messages DROP COLUMN IF EXISTS client_message_id;
