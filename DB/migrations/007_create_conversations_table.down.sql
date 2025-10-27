DROP TRIGGER IF EXISTS update_conversations_updated_at ON conversations;
DROP INDEX IF EXISTS idx_messages_created_at;
DROP INDEX IF EXISTS idx_messages_conversation_id;
DROP INDEX IF EXISTS idx_conversations_updated_at;
DROP INDEX IF EXISTS idx_conversations_item_type;
DROP INDEX IF EXISTS idx_conversations_user_id;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS conversations;
