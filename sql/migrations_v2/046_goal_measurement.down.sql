DROP INDEX IF EXISTS idx_check_ins_habit_date;
DROP TABLE IF EXISTS goal_milestones;
ALTER TABLE goals DROP CONSTRAINT IF EXISTS goals_numeric_target_required;
ALTER TABLE goals
  DROP COLUMN IF EXISTS unit,
  DROP COLUMN IF EXISTS target_value,
  DROP COLUMN IF EXISTS current_value,
  DROP COLUMN IF EXISTS start_value,
  DROP COLUMN IF EXISTS measurement;
