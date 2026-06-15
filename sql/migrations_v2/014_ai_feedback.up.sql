-- AI-generated feedback for a check-in (at most one per check-in).
CREATE TABLE ai_feedback (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    check_in_id uuid NOT NULL UNIQUE REFERENCES check_ins(id) ON DELETE CASCADE,
    habit_id    uuid NOT NULL REFERENCES habits(id) ON DELETE CASCADE,
    content     text NOT NULL,
    model       text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_ai_feedback_user ON ai_feedback (user_id, created_at DESC);
