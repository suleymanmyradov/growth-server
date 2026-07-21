-- Add 'ai_feedback' to the notifications.type CHECK constraint.
-- Migration 016 created the constraint inline (unnamed), so PostgreSQL
-- auto-generated the name `notifications_type_check` (<table>_<column>_check).
-- We drop and recreate it additively; 016 itself is left immutable.
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
            'system',
            'ai_feedback'
        )
    );
