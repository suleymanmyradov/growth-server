-- Add CHECK constraints for data validation

-- Habits: ensure streak is non-negative
ALTER TABLE habits ADD CONSTRAINT chk_habits_streak_non_negative CHECK (streak >= 0);

-- Habits: limit description length to prevent abuse
ALTER TABLE habits ADD CONSTRAINT chk_habits_description_length CHECK (length(description) <= 5000);

-- Articles: ensure read_time is positive
ALTER TABLE articles ADD CONSTRAINT chk_articles_read_time_positive CHECK (read_time > 0);

-- Articles: ensure title is not empty
ALTER TABLE articles ADD CONSTRAINT chk_articles_title_not_empty CHECK (length(trim(title)) > 0);
