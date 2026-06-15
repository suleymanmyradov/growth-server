CREATE TABLE saved_goals (
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    goal_id    uuid NOT NULL REFERENCES goals(id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),

    UNIQUE (goal_id, user_id)
);

CREATE INDEX idx_saved_goals_user ON saved_goals (user_id, created_at DESC);
