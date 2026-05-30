-- Covering index for full-row user_settings lookups (avoids heap fetches)
CREATE INDEX IF NOT EXISTS idx_user_settings_user_id_covering ON user_settings(user_id, theme, language, timezone, email_notifications, push_notifications, habit_reminders, goal_reminders, accountability_style, check_in_time, onboarding_completed, created_at, updated_at, version);
