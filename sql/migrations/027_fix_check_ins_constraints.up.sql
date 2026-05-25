-- Clean up invalid blocker values before adding constraint
UPDATE check_ins
SET blocker = NULL
WHERE blocker IS NOT NULL AND blocker NOT IN ('lack_of_time', 'low_motivation', 'too_distracted', 'unclear_plan', 'other');

-- Add CHECK constraint on blocker column to enforce valid values
-- Use NOT VALID to allow existing data, then validate in a separate step
ALTER TABLE check_ins
ADD CONSTRAINT check_ins_blocker_check
CHECK (blocker IS NULL OR blocker IN ('lack_of_time', 'low_motivation', 'too_distracted', 'unclear_plan', 'other'))
NOT VALID;

-- Validate the constraint (this can be done separately without blocking writes)
ALTER TABLE check_ins
VALIDATE CONSTRAINT check_ins_blocker_check;

-- Add comment documenting that check_ins are intentionally immutable (no updated_at)
COMMENT ON TABLE check_ins IS 'Check-in records are immutable events - they have no updated_at column and should never be modified after creation';

-- Note: The timezone-aware unique constraint is added in migration 028
-- which adds a local_date column to properly handle user timezones
