-- Drop the stored habits.streak column. Streaks are now derived from
-- check_ins history (consecutive completed days in the owner's timezone) by
-- the GetHabitStreak / GetHabitStreaks queries; nothing reads or writes the
-- stored counter anymore.
ALTER TABLE habits DROP COLUMN streak;
