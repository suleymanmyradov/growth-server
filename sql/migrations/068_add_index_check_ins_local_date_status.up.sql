-- Index for GetTodayCheckIns queries by user_id + local_date + status
CREATE INDEX IF NOT EXISTS idx_check_ins_user_local_date_status ON check_ins(user_id, local_date, status);
