-- Typed goal progress: goals.progress stops being a user-typed number and
-- becomes a value derived from a per-goal measurement type. Existing rows
-- become measurement='manual' (zero behavior change on deploy).

ALTER TABLE goals
  ADD COLUMN measurement  text NOT NULL DEFAULT 'manual'
    CHECK (measurement IN ('binary','numeric','milestone','habit','manual')),
  ADD COLUMN start_value   numeric(14,2) NOT NULL DEFAULT 0,
  ADD COLUMN current_value numeric(14,2) NOT NULL DEFAULT 0,
  ADD COLUMN target_value  numeric(14,2),
  ADD COLUMN unit          varchar(32);

ALTER TABLE goals ADD CONSTRAINT goals_numeric_target_required
  CHECK (measurement <> 'numeric' OR (target_value IS NOT NULL AND target_value <> start_value));

-- Milestone steps for measurement='milestone' goals.
CREATE TABLE goal_milestones (
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    goal_id    uuid NOT NULL REFERENCES goals(id) ON DELETE CASCADE,
    title      varchar(200) NOT NULL CHECK (length(trim(title)) > 0),
    sort_order integer NOT NULL DEFAULT 0,
    done_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_goal_milestones_goal ON goal_milestones (goal_id, sort_order);

-- Habit-driven progress recompute reads check_ins by habit + date range.
CREATE INDEX idx_check_ins_habit_date
  ON check_ins (habit_id, local_date) WHERE status = 'completed';
