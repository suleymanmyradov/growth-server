-- Add partial index for missed check-in blocker queries to improve performance
CREATE INDEX idx_check_ins_missed_blocker
    ON check_ins(user_id, created_at)
    WHERE status = 'missed' AND blocker IS NOT NULL;
