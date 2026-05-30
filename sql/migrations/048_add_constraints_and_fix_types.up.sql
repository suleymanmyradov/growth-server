-- Fix user_settings.check_in_time missing NOT NULL (has DEFAULT but allows NULL)
-- Step 1: Add DEFAULT first to prevent race conditions with concurrent INSERTs
ALTER TABLE user_settings ALTER COLUMN check_in_time SET DEFAULT '09:00:00';

-- Step 2: Backfill existing NULL values
UPDATE user_settings SET check_in_time = '09:00:00' WHERE check_in_time IS NULL;

-- Step 3: Add NOT NULL constraint
ALTER TABLE user_settings ALTER COLUMN check_in_time SET NOT NULL;

-- Add state-validity CHECK constraint to user_subscriptions
-- Active or trialing subscriptions must have billing interval and period dates
ALTER TABLE user_subscriptions
ADD CONSTRAINT chk_subscription_active_has_dates
CHECK (
    status NOT IN ('active', 'trialing') OR (
        billing_interval IS NOT NULL AND
        current_period_start IS NOT NULL AND
        current_period_end IS NOT NULL
    )
);

-- Convert upgrade_events.billing_interval to enum type (was missed in migration 038)
-- Sanitize any invalid values first
UPDATE upgrade_events
SET billing_interval = NULL
WHERE billing_interval IS NOT NULL AND billing_interval NOT IN ('monthly', 'annual');

ALTER TABLE upgrade_events
ALTER COLUMN billing_interval TYPE billing_interval_type
USING billing_interval::billing_interval_type;
