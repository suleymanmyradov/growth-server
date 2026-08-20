-- Tracks consecutive missed check-ins per habit, maintained by the
-- missed-day-recovery consumer in the client service. When the counter
-- crosses a threshold (default 3), a plan_adjustment suggestion is created
-- automatically with source='missed_day_recovery' and type='reduce_difficulty'.
--
-- Owned by the client service (check-ins domain).
CREATE TABLE habit_missed_streaks (
    habit_id                uuid PRIMARY KEY REFERENCES habits(id) ON DELETE CASCADE,
    user_id                 uuid NOT NULL,
    consecutive_missed_days integer NOT NULL DEFAULT 0,
    last_completed_date     date,
    last_recovery_triggered date,  -- date of the last auto-suggestion, to avoid spamming
    updated_at              timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_habit_missed_streaks_user ON habit_missed_streaks (user_id);

CREATE TRIGGER habit_missed_streaks_set_updated_at
    BEFORE UPDATE ON habit_missed_streaks
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
