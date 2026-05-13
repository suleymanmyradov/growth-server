ALTER TABLE user_settings
  ADD COLUMN IF NOT EXISTS accountability_style VARCHAR(20) NOT NULL DEFAULT 'balanced'
    CHECK (accountability_style IN ('gentle', 'balanced', 'strict')),
  ADD COLUMN IF NOT EXISTS check_in_time TIME DEFAULT '09:00:00',
  ADD COLUMN IF NOT EXISTS onboarding_completed BOOLEAN NOT NULL DEFAULT FALSE;
