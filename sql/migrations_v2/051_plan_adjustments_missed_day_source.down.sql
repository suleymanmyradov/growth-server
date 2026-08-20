ALTER TABLE plan_adjustments DROP CONSTRAINT IF EXISTS plan_adjustments_source_check;
ALTER TABLE plan_adjustments ADD CONSTRAINT plan_adjustments_source_check
    CHECK (source IN ('check_in', 'weekly_review', 'assistant', 'pattern_analysis'));
