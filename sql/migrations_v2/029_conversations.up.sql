-- Conversations and their messages for the AI coach chat.
-- Conversations group multi-turn chat sessions; messages store the
-- user/assistant exchange history.

CREATE TABLE conversations (
    id           uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title        varchar(255) NOT NULL DEFAULT '',
    type         varchar(50)  NOT NULL DEFAULT 'coach'
        CHECK (type IN ('coach', 'therapist')),
    last_message text         NOT NULL DEFAULT '',
    created_at   timestamptz  NOT NULL DEFAULT now(),
    updated_at   timestamptz  NOT NULL DEFAULT now()
);

CREATE TRIGGER conversations_set_updated_at
    BEFORE UPDATE ON conversations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE INDEX idx_conversations_user_id ON conversations (user_id, updated_at DESC);

CREATE TABLE conversation_messages (
    id              uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    conversation_id uuid NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    role            varchar(20) NOT NULL DEFAULT 'user'
        CHECK (role IN ('user', 'assistant')),
    content         text NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_conversation_messages_conversation_id ON conversation_messages (conversation_id, created_at);
