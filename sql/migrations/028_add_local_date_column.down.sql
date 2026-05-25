-- Drop the timezone-aware unique index
DROP INDEX IF EXISTS uniq_check_ins_user_habit_local_date;

-- Restore the old UTC-based unique index (if migration 027 was applied)
CREATE UNIQUE INDEX IF NOT EXISTS uniq_check_ins_user_habit_date
ON check_ins(user_id, habit_id, DATE(created_at AT TIME ZONE 'UTC'));

-- Drop the local_date column
ALTER TABLE check_ins
DROP COLUMN IF EXISTS local_date;
