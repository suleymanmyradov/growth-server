-- Revert: restore surrogate UUID key
ALTER TABLE goal_habit_relations DROP CONSTRAINT IF EXISTS goal_habit_relations_pkey;
ALTER TABLE goal_habit_relations ADD COLUMN id UUID DEFAULT uuid_generate_v7() NOT NULL;
ALTER TABLE goal_habit_relations ADD PRIMARY KEY (id);
ALTER TABLE goal_habit_relations ALTER COLUMN id DROP DEFAULT;

-- Re-add single-column indexes
CREATE INDEX idx_goal_habit_relations_goal_id ON goal_habit_relations(goal_id);
CREATE INDEX idx_goal_habit_relations_habit_id ON goal_habit_relations(habit_id);
