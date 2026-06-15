-- Scheduled reminder queue. sent_at IS NULL = pending.
CREATE TABLE reminders (
    id           uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type         text NOT NULL CHECK (type IN (
                     'habit_reminder', 'missed_check_in', 'weekly_review', 'encouragement')),
    scheduled_at timestamptz NOT NULL,
    sent_at      timestamptz,
    metadata     jsonb NOT NULL DEFAULT '{}',
    created_at   timestamptz NOT NULL DEFAULT now()
);

-- worker poll: due, unsent reminders
CREATE INDEX idx_reminders_due ON reminders (scheduled_at) WHERE sent_at IS NULL;
-- at most one pending reminder per user/type/day
CREATE UNIQUE INDEX uniq_reminders_pending_per_day
    ON reminders (user_id, type, (((scheduled_at AT TIME ZONE 'UTC'))::date))
    WHERE sent_at IS NULL;
