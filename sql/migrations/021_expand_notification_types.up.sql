ALTER TABLE notifications DROP CONSTRAINT notifications_item_type_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_item_type_check
  CHECK (item_type IN ('habit_reminder','missed_check_in','goal_deadline',
                       'achievement','weekly_review','encouragement','system'));
