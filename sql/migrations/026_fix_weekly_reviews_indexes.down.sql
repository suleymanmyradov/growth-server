-- Remove partial index for missed check-in blocker queries
DROP INDEX IF EXISTS idx_check_ins_missed_blocker;
