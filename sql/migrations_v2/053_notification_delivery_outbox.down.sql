ALTER TABLE notification_devices
    DROP CONSTRAINT IF EXISTS notification_devices_installation_id_key;

ALTER TABLE notification_devices
    ADD CONSTRAINT notification_devices_installation_id_user_id_key UNIQUE (installation_id, user_id);

ALTER TABLE reminders
    DROP CONSTRAINT IF EXISTS reminders_type_check;

ALTER TABLE reminders
    ADD CONSTRAINT reminders_type_check CHECK (
        type IN (
            'habit_reminder',
            'missed_check_in',
            'weekly_review',
            'encouragement',
            'coach_digest'
        )
    );

DROP TABLE IF EXISTS notification_habit_state;
DROP TABLE IF EXISTS notification_recipients;
DROP TABLE IF EXISTS notification_deliveries;

DROP INDEX IF EXISTS idx_notifications_deduplication_key;

ALTER TABLE notifications
    DROP CONSTRAINT IF EXISTS notifications_destination_check;

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

ALTER TABLE notifications
    DROP COLUMN IF EXISTS metadata,
    DROP COLUMN IF EXISTS deduplication_key,
    DROP COLUMN IF EXISTS resource_id,
    DROP COLUMN IF EXISTS destination;
