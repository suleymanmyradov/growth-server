-- Add local_date column to store user's local date at insert time
-- This enables timezone-aware unique constraint without functional index limitations
ALTER TABLE check_ins
ADD COLUMN IF NOT EXISTS local_date DATE;

-- Populate local_date for existing rows using user's timezone
-- Only populate rows where local_date is NULL to avoid re-processing
UPDATE check_ins ci
SET local_date = (ci.created_at AT TIME ZONE us.timezone)::date
FROM user_settings us
WHERE ci.user_id = us.user_id
  AND ci.local_date IS NULL;

-- Make local_date NOT NULL after populating existing data
ALTER TABLE check_ins
ALTER COLUMN local_date SET NOT NULL;

-- Drop the old UTC-based unique index (if it exists from migration 027)
DROP INDEX IF EXISTS uniq_check_ins_user_habit_date;

-- Create timezone-aware unique index on local_date
CREATE UNIQUE INDEX IF NOT EXISTS uniq_check_ins_user_habit_local_date
ON check_ins(user_id, habit_id, local_date);

-- Add comment explaining local_date purpose
COMMENT ON COLUMN check_ins.local_date IS 'User''s local date at time of check-in, used for timezone-aware deduplication';
