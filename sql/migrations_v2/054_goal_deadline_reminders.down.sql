-- Reverse migration for 054_goal_deadline_reminders.

-- Drop the goal read model.
DROP TABLE IF EXISTS notification_goal_state;

-- Drop the per-goal unique index.
DROP INDEX IF EXISTS uniq_reminders_goal_deadline_per_goal;

-- Restore the original per-day unique index (no type exclusion).
DROP INDEX IF EXISTS uniq_reminders_pending_per_day;
CREATE UNIQUE INDEX uniq_reminders_pending_per_day
    ON reminders (user_id, type, (((scheduled_at AT TIME ZONE 'UTC'))::date))
    WHERE sent_at IS NULL;

-- Remove goal_deadline from the reminders type CHECK.
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
