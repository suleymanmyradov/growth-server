-- Partial index for ResetTodayHabits (updates only habits where completed = TRUE)
CREATE INDEX IF NOT EXISTS idx_habits_user_completed ON habits(user_id) WHERE completed = TRUE;
