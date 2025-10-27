DROP TRIGGER IF EXISTS update_goals_updated_at ON goals;
DROP INDEX IF EXISTS idx_goal_habit_relations_habit_id;
DROP INDEX IF EXISTS idx_goal_habit_relations_goal_id;
DROP INDEX IF EXISTS idx_goals_due_date;
DROP INDEX IF EXISTS idx_goals_completed;
DROP INDEX IF EXISTS idx_goals_category;
DROP INDEX IF EXISTS idx_goals_user_id;
DROP TABLE IF EXISTS goal_habit_relations;
DROP TABLE IF EXISTS goals;
