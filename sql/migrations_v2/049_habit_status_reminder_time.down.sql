DROP INDEX IF EXISTS idx_habits_user_active;
ALTER TABLE habits
    DROP COLUMN IF EXISTS reminder_time,
    DROP COLUMN IF EXISTS status;
