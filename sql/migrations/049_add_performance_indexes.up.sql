-- Performance indexes for the top 5 critical query optimizations.
-- These support the rewritten queries and eliminate remaining table scans.

-- Fix #2 (saved_items UNION ALL): composite indexes so each arm can satisfy
-- ORDER BY created_at DESC LIMIT N with an index-only scan.
CREATE INDEX IF NOT EXISTS idx_saved_articles_user_created_desc
    ON saved_articles(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_saved_goals_user_created_desc
    ON saved_goals(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_saved_habits_user_created_desc
    ON saved_habits(user_id, created_at DESC);

-- Fix #4 (activities feed): DESC composite index eliminates Sort node
-- for ListActivities, ListActivitiesByUser, GetActivityFeed.
CREATE INDEX IF NOT EXISTS idx_activities_user_created_desc
    ON activities(user_id, created_at DESC);

-- Fix #5 (GetStreaks): partial index on completed check-ins so DISTINCT
-- local_date lookups are index-only and tiny.
CREATE INDEX IF NOT EXISTS idx_check_ins_user_completed_local
    ON check_ins(user_id, local_date)
    WHERE status = 'completed';

-- Bonus: partial index for weekly review blocker stats (GetBlockerStatsForWeek)
CREATE INDEX IF NOT EXISTS idx_check_ins_missed_blocker
    ON check_ins(user_id, local_date, blocker)
    WHERE status = 'missed' AND blocker IS NOT NULL;

-- Bonus: partial index for ResetTodayHabits (habits where completed = TRUE)
CREATE INDEX IF NOT EXISTS idx_habits_user_completed
    ON habits(user_id)
    WHERE completed = TRUE;

-- Bonus: index-only scan support for ClaimDueReminders job queue
CREATE INDEX IF NOT EXISTS idx_reminder_queue_due_id
    ON reminder_queue(scheduled_at, id)
    WHERE sent = FALSE;
