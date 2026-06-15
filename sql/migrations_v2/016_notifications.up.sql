CREATE TABLE notifications (
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type       text NOT NULL CHECK (type IN (
                   'habit_reminder', 'missed_check_in', 'goal_deadline',
                   'achievement', 'weekly_review', 'encouragement', 'system')),
    title      varchar(200) NOT NULL,
    message    text NOT NULL,
    is_read    boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_notifications_user ON notifications (user_id, created_at DESC);
-- unread badge counts
CREATE INDEX idx_notifications_unread ON notifications (user_id) WHERE is_read = false;
