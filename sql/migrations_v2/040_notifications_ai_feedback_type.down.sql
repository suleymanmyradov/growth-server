-- Revert notifications.type back to the original 016 set (no ai_feedback).
ALTER TABLE notifications
    DROP CONSTRAINT IF EXISTS notifications_type_check;

ALTER TABLE notifications
    ADD CONSTRAINT notifications_type_check CHECK (
        type IN (
            'habit_reminder',
            'missed_check_in',
            'goal_deadline',
            'achievement',
            'weekly_review',
            'encouragement',
            'system'
        )
    );
