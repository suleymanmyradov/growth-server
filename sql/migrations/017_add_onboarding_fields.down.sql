ALTER TABLE user_settings
  DROP COLUMN IF EXISTS accountability_style,
  DROP COLUMN IF EXISTS check_in_time,
  DROP COLUMN IF EXISTS onboarding_completed;
