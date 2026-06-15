-- App preferences + coaching preferences (old `user_coaching_profiles` merged in).
-- One row per user; user_id is the PK so no separate id is needed.
CREATE TABLE user_settings (
    user_id              uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    theme                text NOT NULL DEFAULT 'system' CHECK (theme IN ('light', 'dark', 'system')),
    language             varchar(10) NOT NULL DEFAULT 'en',
    timezone             varchar(50) NOT NULL DEFAULT 'UTC',
    email_notifications  boolean NOT NULL DEFAULT true,
    push_notifications   boolean NOT NULL DEFAULT true,
    habit_reminders      boolean NOT NULL DEFAULT true,
    goal_reminders       boolean NOT NULL DEFAULT true,
    check_in_time        time NOT NULL DEFAULT '09:00',
    onboarding_completed boolean NOT NULL DEFAULT false,

    -- coaching preferences
    accountability_style text NOT NULL DEFAULT 'balanced'
        CHECK (accountability_style IN ('gentle', 'balanced', 'strict')),
    coach_tone           text NOT NULL DEFAULT 'supportive'
        CHECK (coach_tone IN ('supportive', 'direct', 'warm', 'practical', 'challenging')),
    difficulty           text NOT NULL DEFAULT 'adaptive'
        CHECK (difficulty IN ('easy', 'adaptive', 'ambitious')),
    primary_motivation   text,
    common_blockers      jsonb NOT NULL DEFAULT '[]',
    coaching_notes       jsonb NOT NULL DEFAULT '{}',
    last_context_refresh_at timestamptz,

    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER user_settings_set_updated_at
    BEFORE UPDATE ON user_settings
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
