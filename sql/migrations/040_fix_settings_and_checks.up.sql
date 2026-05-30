-- Fix user_settings boolean flags to be NOT NULL (three-valued logic bug)
-- Step 1: Add DEFAULT first to prevent race conditions with concurrent INSERTs
ALTER TABLE user_settings
    ALTER COLUMN email_notifications SET DEFAULT TRUE,
    ALTER COLUMN push_notifications SET DEFAULT TRUE,
    ALTER COLUMN habit_reminders SET DEFAULT TRUE,
    ALTER COLUMN goal_reminders SET DEFAULT TRUE;

-- Step 2: Backfill existing NULL values
UPDATE user_settings SET email_notifications = TRUE WHERE email_notifications IS NULL;
UPDATE user_settings SET push_notifications = TRUE WHERE push_notifications IS NULL;
UPDATE user_settings SET habit_reminders = TRUE WHERE habit_reminders IS NULL;
UPDATE user_settings SET goal_reminders = TRUE WHERE goal_reminders IS NULL;

-- Step 3: Add NOT NULL constraints
ALTER TABLE user_settings
    ALTER COLUMN email_notifications SET NOT NULL,
    ALTER COLUMN push_notifications SET NOT NULL,
    ALTER COLUMN habit_reminders SET NOT NULL,
    ALTER COLUMN goal_reminders SET NOT NULL;

-- Enforce check_ins immutability at the database level
CREATE OR REPLACE FUNCTION prevent_check_in_update()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'check_ins are immutable events and cannot be updated';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER check_ins_no_update
BEFORE UPDATE ON check_ins
FOR EACH ROW EXECUTE FUNCTION prevent_check_in_update();

-- Add username format check to prevent invalid characters
ALTER TABLE users
ADD CONSTRAINT chk_username_format
CHECK (username ~* '^[a-z0-9_-]+$');
