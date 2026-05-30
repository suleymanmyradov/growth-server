-- Revert ENUM types back to plain text (CHECK constraints re-added in migration 038 down)

DROP TYPE IF EXISTS check_in_status CASCADE;
DROP TYPE IF EXISTS mood_type CASCADE;
DROP TYPE IF EXISTS energy_level CASCADE;
DROP TYPE IF EXISTS blocker_type CASCADE;
DROP TYPE IF EXISTS notification_type CASCADE;
DROP TYPE IF EXISTS activity_type CASCADE;
DROP TYPE IF EXISTS conversation_type CASCADE;
DROP TYPE IF EXISTS saved_item_type CASCADE;
DROP TYPE IF EXISTS theme_type CASCADE;
DROP TYPE IF EXISTS accountability_style_type CASCADE;
DROP TYPE IF EXISTS coach_tone_type CASCADE;
DROP TYPE IF EXISTS difficulty_level_type CASCADE;
DROP TYPE IF EXISTS subscription_status_type CASCADE;
DROP TYPE IF EXISTS billing_interval_type CASCADE;
DROP TYPE IF EXISTS upgrade_event_type CASCADE;
DROP TYPE IF EXISTS reminder_type CASCADE;
DROP TYPE IF EXISTS plan_adjustment_status_type CASCADE;
DROP TYPE IF EXISTS plan_adjustment_source_type CASCADE;
DROP TYPE IF EXISTS plan_adjustment_type_type CASCADE;
DROP TYPE IF EXISTS message_role_type CASCADE;
