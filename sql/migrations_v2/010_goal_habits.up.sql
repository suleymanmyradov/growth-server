-- Many-to-many link between goals and habits.
CREATE TABLE goal_habits (
    goal_id    uuid NOT NULL REFERENCES goals(id) ON DELETE CASCADE,
    habit_id   uuid NOT NULL REFERENCES habits(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (goal_id, habit_id)
);

CREATE INDEX idx_goal_habits_habit ON goal_habits (habit_id);
