-- Revert changes
ALTER TABLE user_settings
    ALTER COLUMN email_notifications DROP NOT NULL,
    ALTER COLUMN push_notifications DROP NOT NULL,
    ALTER COLUMN habit_reminders DROP NOT NULL,
    ALTER COLUMN goal_reminders DROP NOT NULL;

DROP TRIGGER IF EXISTS check_ins_no_update ON check_ins;
DROP FUNCTION IF EXISTS prevent_check_in_update();

ALTER TABLE users DROP CONSTRAINT IF EXISTS chk_username_format;
