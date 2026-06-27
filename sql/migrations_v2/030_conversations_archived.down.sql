DROP INDEX IF EXISTS idx_conversations_user_archived;
ALTER TABLE conversations DROP COLUMN IF EXISTS archived;
