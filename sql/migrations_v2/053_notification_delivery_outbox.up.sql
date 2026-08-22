ALTER TABLE notifications
    ADD COLUMN destination text,
    ADD COLUMN resource_id uuid,
    ADD COLUMN deduplication_key text,
    ADD COLUMN metadata jsonb NOT NULL DEFAULT '{}';

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
            'ai_feedback',
            'streak_warning'
        )
    );

ALTER TABLE notifications
    ADD CONSTRAINT notifications_destination_check CHECK (
        destination IS NULL OR destination IN (
            'habit-detail',
            'goal-detail',
            'article-detail',
            'conversation',
            'weekly-review',
            'activity',
            'notifications'
        )
    );

CREATE UNIQUE INDEX idx_notifications_deduplication_key
    ON notifications (deduplication_key)
    WHERE deduplication_key IS NOT NULL;

CREATE TABLE notification_deliveries (
    id                  uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    notification_id     uuid NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
    user_id             uuid NOT NULL,
    channel             text NOT NULL CHECK (channel IN ('push', 'email')),
    status              text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'sent', 'failed', 'suppressed')),
    attempt_count       integer NOT NULL DEFAULT 0,
    next_attempt_at     timestamptz NOT NULL DEFAULT now(),
    claimed_at          timestamptz,
    provider_message_id text,
    last_error_code     text,
    last_error_message  text,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    sent_at             timestamptz,
    UNIQUE (notification_id, channel)
);

CREATE INDEX idx_notification_deliveries_pending
    ON notification_deliveries (next_attempt_at, created_at)
    WHERE status = 'pending';

CREATE INDEX idx_notification_deliveries_user
    ON notification_deliveries (user_id, created_at DESC);

CREATE TRIGGER notification_deliveries_set_updated_at
    BEFORE UPDATE ON notification_deliveries
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE notification_recipients (
    user_id        uuid PRIMARY KEY,
    email          text NOT NULL DEFAULT '',
    name           text NOT NULL DEFAULT '',
    email_verified boolean NOT NULL DEFAULT false,
    updated_at     timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE notification_habit_state (
    user_id                  uuid NOT NULL,
    habit_id                 uuid NOT NULL,
    habit_name               text NOT NULL,
    current_streak           integer NOT NULL DEFAULT 0 CHECK (current_streak >= 0),
    last_check_in_local_date date,
    active                   boolean NOT NULL DEFAULT true,
    updated_at               timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, habit_id)
);

CREATE INDEX idx_notification_habit_state_at_risk
    ON notification_habit_state (user_id, current_streak DESC)
    WHERE active = true AND current_streak > 0;

ALTER TABLE reminders
    DROP CONSTRAINT IF EXISTS reminders_type_check;

ALTER TABLE reminders
    ADD CONSTRAINT reminders_type_check CHECK (
        type IN (
            'habit_reminder',
            'missed_check_in',
            'weekly_review',
            'encouragement',
            'coach_digest',
            'streak_warning_scan'
        )
    );

ALTER TABLE notification_devices
    DROP CONSTRAINT IF EXISTS notification_devices_installation_id_user_id_key;

ALTER TABLE notification_devices
    ADD CONSTRAINT notification_devices_installation_id_key UNIQUE (installation_id);
