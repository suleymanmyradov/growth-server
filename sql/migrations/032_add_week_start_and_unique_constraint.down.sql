-- Remove index for week_start
DROP INDEX IF EXISTS idx_plan_adjustment_suggestions_week_start;

-- Remove unique constraint
ALTER TABLE plan_adjustment_suggestions 
DROP CONSTRAINT IF EXISTS plan_adjustment_suggestions_unique_key;

-- Remove week_start column
ALTER TABLE plan_adjustment_suggestions 
DROP COLUMN IF EXISTS week_start;
