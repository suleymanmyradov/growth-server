-- Goal deadline reminders: allow scheduling per-goal deadline reminders in
-- the reminders queue, and maintain a local goal read model in the
-- notifications service so goal-deadline reminders can be rescheduled when
-- the user toggles the goalReminders preference back on.
--
-- 1. Add 'goal_deadline' to the reminders.type CHECK constraint.
-- 2. Replace uniq_reminders_pending_per_day so it excludes goal_deadline
--    (a user can have multiple goals due the same day, each with its own
--    reminder). The ON CONFLICT in EnqueueReminder is updated to match.
-- 3. Add a per-goal unique index for goal_deadline reminders so re-enqueuing
--    the same goal's deadline upserts instead of duplicating.
-- 4. Create notification_goal_state: a local read model maintained from
--    goal_created/goal_updated/goal_deleted/goal_completed events.

-- 1. reminders type: add goal_deadline
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
            'streak_warning_scan',
            'goal_deadline'
        )
    );

-- 2. Replace per-day unique index, excluding goal_deadline.
DROP INDEX IF EXISTS uniq_reminders_pending_per_day;
CREATE UNIQUE INDEX uniq_reminders_pending_per_day
    ON reminders (user_id, type, (((scheduled_at AT TIME ZONE 'UTC'))::date))
    WHERE sent_at IS NULL AND type <> 'goal_deadline';

-- 3. One pending goal_deadline reminder per (user, goal).
CREATE UNIQUE INDEX uniq_reminders_goal_deadline_per_goal
    ON reminders (user_id, (metadata->>'goalId'))
    WHERE sent_at IS NULL AND type = 'goal_deadline';

-- 4. Local goal read model for the notifications service.
CREATE TABLE notification_goal_state (
    goal_id    uuid PRIMARY KEY,
    user_id    uuid NOT NULL,
    title      text NOT NULL,
    deadline   timestamptz,
    completed  boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Look up upcoming deadlines for a user (for rescheduling on re-enable).
CREATE INDEX idx_notification_goal_state_user_deadline
    ON notification_goal_state (user_id, deadline)
    WHERE NOT completed AND deadline IS NOT NULL;
