CREATE TABLE saved_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    item_type VARCHAR(20) NOT NULL CHECK (item_type IN ('article', 'goal', 'habit')),
    item_id UUID NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, item_type, item_id)
);

CREATE INDEX idx_saved_items_user_id ON saved_items(user_id);
CREATE INDEX idx_saved_items_item_type ON saved_items(item_type);
CREATE INDEX idx_saved_items_created_at ON saved_items(created_at);
