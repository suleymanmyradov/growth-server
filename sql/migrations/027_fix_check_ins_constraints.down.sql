-- Remove the comment
COMMENT ON TABLE check_ins IS NULL;

-- Remove the CHECK constraint on blocker
ALTER TABLE check_ins DROP CONSTRAINT IF EXISTS check_ins_blocker_check;
