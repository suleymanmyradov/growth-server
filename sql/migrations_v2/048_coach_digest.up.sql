-- Coach daily digest: replace per-check-in AI feedback with one daily digest.
--
-- 1. Add 'coach_digest' to the reminders.type CHECK constraint so the
--    notifications service can schedule a daily digest reminder.
-- 2. Make ai_feedback.check_in_id and habit_id nullable so a digest row
--    (which covers multiple check-ins, not one) can be stored with NULLs.
--    Replace the UNIQUE constraint on check_in_id with a partial unique
--    index that only applies when check_in_id IS NOT NULL, preserving
--    the "at most one feedback per check-in" guarantee for legacy rows.

-- 1. reminders type: add coach_digest
ALTER TABLE reminders
    DROP CONSTRAINT IF EXISTS reminders_type_check;

ALTER TABLE reminders
    ADD CONSTRAINT reminders_type_check CHECK (
        type IN (
            'habit_reminder',
            'missed_check_in',
            'weekly_review',
            'encouragement',
            'coach_digest'
        )
    );

-- 2. ai_feedback: allow nullable check_in_id / habit_id for digest rows
ALTER TABLE ai_feedback
    ALTER COLUMN check_in_id DROP NOT NULL;

ALTER TABLE ai_feedback
    ALTER COLUMN habit_id DROP NOT NULL;

-- Replace the UNIQUE CONSTRAINT on check_in_id with a partial unique index
-- so NULL check_in_id (digest rows) don't conflict, but non-NULL values
-- remain unique (at most one feedback per check-in).
ALTER TABLE ai_feedback
    DROP CONSTRAINT IF EXISTS ai_feedback_check_in_id_key;

CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_feedback_check_in_id_unique
    ON ai_feedback (check_in_id)
    WHERE check_in_id IS NOT NULL;
