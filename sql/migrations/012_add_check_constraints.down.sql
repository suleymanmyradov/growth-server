-- Remove CHECK constraints

ALTER TABLE articles DROP CONSTRAINT IF EXISTS chk_articles_title_not_empty;
ALTER TABLE articles DROP CONSTRAINT IF EXISTS chk_articles_read_time_positive;
ALTER TABLE habits DROP CONSTRAINT IF EXISTS chk_habits_description_length;
ALTER TABLE habits DROP CONSTRAINT IF EXISTS chk_habits_streak_non_negative;
