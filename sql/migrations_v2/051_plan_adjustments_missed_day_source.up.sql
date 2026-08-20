-- Add 'missed_day_recovery' as a valid source for plan_adjustments.
-- This source is used by the check-in events consumer when a habit crosses
-- the consecutive-missed-day threshold.
ALTER TABLE plan_adjustments DROP CONSTRAINT IF EXISTS plan_adjustments_source_check;
ALTER TABLE plan_adjustments ADD CONSTRAINT plan_adjustments_source_check
    CHECK (source IN ('check_in', 'weekly_review', 'assistant', 'pattern_analysis', 'missed_day_recovery'));
