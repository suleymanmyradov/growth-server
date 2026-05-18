ALTER TABLE notifications DROP CONSTRAINT notifications_item_type_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_item_type_check
  CHECK (item_type IN ('habit_reminder', 'goal_deadline', 'achievement', 'system'));
