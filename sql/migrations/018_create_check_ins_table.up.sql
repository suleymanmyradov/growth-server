CREATE TABLE check_ins (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    habit_id UUID NOT NULL REFERENCES habits(id) ON DELETE CASCADE,
    status VARCHAR(10) NOT NULL CHECK (status IN ('completed', 'missed')),
    mood VARCHAR(20) CHECK (mood IN ('great', 'okay', 'low', 'stressed')),
    energy VARCHAR(20) CHECK (energy IN ('high', 'medium', 'low')),
    blocker VARCHAR(50),
    note TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_check_ins_user_id ON check_ins(user_id);
CREATE INDEX idx_check_ins_habit_id ON check_ins(habit_id);
CREATE INDEX idx_check_ins_user_date ON check_ins(user_id, created_at);
CREATE INDEX idx_check_ins_habit_date ON check_ins(habit_id, created_at);
