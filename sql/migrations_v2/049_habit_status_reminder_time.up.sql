-- Adds status (active/paused) and reminder_time columns to habits so that
-- plan adjustment suggestions can actually mutate the habit when applied
-- (reduce_difficulty updates description, change_time updates reminder_time,
-- pause sets status to 'paused'). Defaults preserve existing behavior: all
-- current habits are active with no explicit reminder time.
ALTER TABLE habits
    ADD COLUMN status        text      NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused')),
    ADD COLUMN reminder_time time;

CREATE INDEX idx_habits_user_active ON habits (user_id, status, created_at DESC) WHERE status = 'active';
