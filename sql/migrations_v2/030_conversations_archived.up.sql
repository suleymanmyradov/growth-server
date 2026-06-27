ALTER TABLE conversations
    ADD COLUMN archived boolean NOT NULL DEFAULT false;

CREATE INDEX idx_conversations_user_archived
    ON conversations (user_id, archived, updated_at DESC);
