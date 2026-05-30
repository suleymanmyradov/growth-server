-- Revert upgrade_events.billing_interval to plain VARCHAR
ALTER TABLE upgrade_events
ALTER COLUMN billing_interval TYPE VARCHAR(20);

-- Drop subscription state validity check
ALTER TABLE user_subscriptions
DROP CONSTRAINT IF EXISTS chk_subscription_active_has_dates;

-- Revert user_settings.check_in_time to nullable
ALTER TABLE user_settings ALTER COLUMN check_in_time DROP NOT NULL;
