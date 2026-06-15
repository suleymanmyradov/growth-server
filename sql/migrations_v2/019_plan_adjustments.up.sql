-- AI-suggested plan adjustments. Targets exactly one goal OR one habit
-- (two nullable FKs + XOR check: real referential integrity, no polymorphic table).
CREATE TABLE plan_adjustments (
    id              uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id         uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    goal_id         uuid REFERENCES goals(id) ON DELETE CASCADE,
    habit_id        uuid REFERENCES habits(id) ON DELETE CASCADE,
    source          text NOT NULL CHECK (source IN (
                        'check_in', 'weekly_review', 'assistant', 'pattern_analysis')),
    adjustment_type text NOT NULL CHECK (adjustment_type IN (
                        'reduce_difficulty', 'increase_difficulty', 'change_time',
                        'clarify_plan', 'pause', 'keep_same')),
    status          text NOT NULL DEFAULT 'pending' CHECK (status IN (
                        'pending', 'accepted', 'dismissed', 'applied')),
    reason          text NOT NULL,
    suggestion      text NOT NULL,
    metadata        jsonb NOT NULL DEFAULT '{}',
    week_start      date,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT plan_adjustments_one_target CHECK ((goal_id IS NULL) <> (habit_id IS NULL))
);

CREATE INDEX idx_plan_adjustments_user ON plan_adjustments (user_id, status, created_at DESC);

-- The AI coach re-emits suggestions; this lets inserts upsert instead of duplicating.
CREATE UNIQUE INDEX uniq_plan_adjustments_suggestion
    ON plan_adjustments (user_id, source, adjustment_type, COALESCE(goal_id, habit_id), week_start);

CREATE TRIGGER plan_adjustments_set_updated_at
    BEFORE UPDATE ON plan_adjustments
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
