-- One check-in per habit per local day. Treated as immutable by the app
-- (no updated_at on purpose).
CREATE TABLE check_ins (
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    habit_id   uuid NOT NULL REFERENCES habits(id) ON DELETE CASCADE,
    local_date date NOT NULL,
    status     text NOT NULL CHECK (status IN ('completed', 'missed')),
    mood       text CHECK (mood IN ('great', 'okay', 'low', 'stressed')),
    energy     text CHECK (energy IN ('high', 'medium', 'low')),
    blocker    text CHECK (blocker IN ('lack_of_time', 'low_motivation', 'too_distracted', 'unclear_plan', 'other')),
    note       text,
    created_at timestamptz NOT NULL DEFAULT now(),

    -- a habit belongs to one user, so (habit_id, local_date) is enough
    UNIQUE (habit_id, local_date)
);

CREATE INDEX idx_check_ins_user_date ON check_ins (user_id, local_date DESC);
