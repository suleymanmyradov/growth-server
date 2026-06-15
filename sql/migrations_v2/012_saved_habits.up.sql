CREATE TABLE saved_habits (
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    habit_id   uuid NOT NULL REFERENCES habits(id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),

    UNIQUE (habit_id, user_id)
);

CREATE INDEX idx_saved_habits_user ON saved_habits (user_id, created_at DESC);
