-- Local, event-fed read model owned by notifications.
-- Replaces the cross-table GetReminderContext query (V2).
CREATE TABLE reminder_state (
    user_id               uuid PRIMARY KEY,
    timezone              varchar(50) NOT NULL DEFAULT 'UTC',
    check_in_time         time NOT NULL DEFAULT '09:00',
    habit_reminders       boolean NOT NULL DEFAULT true,
    onboarding_completed  boolean NOT NULL DEFAULT false,
    active_habit_count    integer NOT NULL DEFAULT 0,
    last_check_in_date    date,
    checked_in_count_today integer NOT NULL DEFAULT 0,
    updated_at            timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER reminder_state_set_updated_at
    BEFORE UPDATE ON reminder_state
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
