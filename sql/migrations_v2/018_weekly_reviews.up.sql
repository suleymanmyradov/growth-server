-- One review per user per week. week_start is the Monday; the end of the
-- week is derivable (week_start + 6), so it is not stored.
CREATE TABLE weekly_reviews (
    id                    uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id               uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    week_start            date NOT NULL,
    total_habits          integer NOT NULL DEFAULT 0,
    completed_check_ins   integer NOT NULL DEFAULT 0,
    missed_check_ins      integer NOT NULL DEFAULT 0,
    completion_rate       numeric(5,2) NOT NULL DEFAULT 0 CHECK (completion_rate BETWEEN 0 AND 100),
    best_day              varchar(20),
    hardest_day           varchar(20),
    top_blocker           varchar(50),
    mood_summary          jsonb NOT NULL DEFAULT '{}',
    energy_summary        jsonb NOT NULL DEFAULT '{}',
    habit_breakdown       jsonb NOT NULL DEFAULT '[]',
    ai_summary            text,
    suggested_adjustments jsonb NOT NULL DEFAULT '[]',
    next_week_plan        jsonb NOT NULL DEFAULT '{}',
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),

    UNIQUE (user_id, week_start)
);

CREATE TRIGGER weekly_reviews_set_updated_at
    BEFORE UPDATE ON weekly_reviews
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
