-- Reverse the coach digest migration.

-- Restore the UNIQUE CONSTRAINT on check_in_id (non-null only).
DROP INDEX IF EXISTS idx_ai_feedback_check_in_id_unique;

ALTER TABLE ai_feedback
    ADD CONSTRAINT ai_feedback_check_in_id_key UNIQUE (check_in_id);

-- Restore NOT NULL on check_in_id and habit_id.
-- This will fail if any digest rows with NULLs exist — delete them first:
--   DELETE FROM ai_feedback WHERE check_in_id IS NULL;
ALTER TABLE ai_feedback
    ALTER COLUMN check_in_id SET NOT NULL;

ALTER TABLE ai_feedback
    ALTER COLUMN habit_id SET NOT NULL;

-- Remove coach_digest from reminders type CHECK.
ALTER TABLE reminders
    DROP CONSTRAINT IF EXISTS reminders_type_check;

ALTER TABLE reminders
    ADD CONSTRAINT reminders_type_check CHECK (
        type IN (
            'habit_reminder',
            'missed_check_in',
            'weekly_review',
            'encouragement'
        )
    );
