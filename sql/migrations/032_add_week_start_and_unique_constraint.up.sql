-- Add week_start column to plan_adjustment_suggestions
ALTER TABLE plan_adjustment_suggestions 
ADD COLUMN week_start DATE;

-- Backfill week_start for existing records from weekly review source
UPDATE plan_adjustment_suggestions 
SET week_start = (
    SELECT wr.week_start 
    FROM weekly_reviews wr 
    WHERE wr.user_id = plan_adjustment_suggestions.user_id 
    AND wr.created_at >= plan_adjustment_suggestions.created_at 
    AND wr.created_at <= plan_adjustment_suggestions.created_at + INTERVAL '7 days'
    ORDER BY ABS(EXTRACT(EPOCH FROM (wr.created_at - plan_adjustment_suggestions.created_at))) ASC
    LIMIT 1
)
WHERE source = 'weekly_review' AND week_start IS NULL;

-- Add unique constraint to prevent duplicates
ALTER TABLE plan_adjustment_suggestions 
ADD CONSTRAINT plan_adjustment_suggestions_unique_key 
UNIQUE (user_id, source, week_start, habit_id, adjustment_type);

-- Add index for week_start
CREATE INDEX idx_plan_adjustment_suggestions_week_start 
ON plan_adjustment_suggestions(week_start);
