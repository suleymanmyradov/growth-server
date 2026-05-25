CREATE TABLE reminder_queue (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type VARCHAR(30) NOT NULL CHECK (type IN
        ('habit_reminder','missed_check_in','weekly_review','encouragement')),
    scheduled_at TIMESTAMPTZ NOT NULL,
    sent BOOLEAN NOT NULL DEFAULT FALSE,
    sent_at TIMESTAMPTZ,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_reminder_queue_due ON reminder_queue(scheduled_at) WHERE sent = FALSE;

CREATE UNIQUE INDEX uniq_reminder_queue_pending_per_day
    ON reminder_queue(user_id, type, ((scheduled_at AT TIME ZONE 'UTC')::date)) WHERE sent = FALSE;

CREATE TRIGGER update_reminder_queue_updated_at
    BEFORE UPDATE ON reminder_queue
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
