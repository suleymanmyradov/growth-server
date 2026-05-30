-- Remove surrogate UUID key from junction table; use composite PK for efficiency
-- The UNIQUE(goal_id, habit_id) already exists and creates the needed composite index.

-- First drop the single-column indexes that become redundant after the composite PK
DROP INDEX IF EXISTS idx_goal_habit_relations_goal_id;
DROP INDEX IF EXISTS idx_goal_habit_relations_habit_id;

-- Drop the surrogate id column and its implicit PK index
ALTER TABLE goal_habit_relations DROP COLUMN IF EXISTS id;

-- Make the natural composite key the primary key
ALTER TABLE goal_habit_relations ADD PRIMARY KEY (goal_id, habit_id);
