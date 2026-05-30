-- Partial index for MarkAllNotificationsRead (updates only unread notifications)
CREATE INDEX IF NOT EXISTS idx_notifications_user_unread ON notifications(user_id) WHERE is_read = FALSE;
