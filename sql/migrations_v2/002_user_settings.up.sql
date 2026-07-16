-- Three tables split from the old user_settings god table, one per owner.
-- All keyed user_id uuid PRIMARY KEY (no FK to users — see V5).

-- client: app-level preferences
CREATE TABLE user_preferences (
    user_id              uuid PRIMARY KEY,
    theme                text NOT NULL DEFAULT 'system' CHECK (theme IN ('light', 'dark', 'system')),
    language             varchar(10) NOT NULL DEFAULT 'en',
    timezone             varchar(50) NOT NULL DEFAULT 'UTC',
    check_in_time        time NOT NULL DEFAULT '09:00',
    onboarding_completed boolean NOT NULL DEFAULT false,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER user_preferences_set_updated_at
    BEFORE UPDATE ON user_preferences
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- notifications: notification channel flags
CREATE TABLE notification_preferences (
    user_id             uuid PRIMARY KEY,
    email_notifications boolean NOT NULL DEFAULT true,
    push_notifications  boolean NOT NULL DEFAULT true,
    habit_reminders     boolean NOT NULL DEFAULT true,
    goal_reminders      boolean NOT NULL DEFAULT true,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER notification_preferences_set_updated_at
    BEFORE UPDATE ON notification_preferences
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ai-coach: coaching preferences
CREATE TABLE coaching_profiles (
    user_id                 uuid PRIMARY KEY,
    accountability_style    text NOT NULL DEFAULT 'balanced'
        CHECK (accountability_style IN ('gentle', 'balanced', 'strict')),
    coach_tone              text NOT NULL DEFAULT 'supportive'
        CHECK (coach_tone IN ('supportive', 'direct', 'warm', 'practical', 'challenging')),
    difficulty              text NOT NULL DEFAULT 'adaptive'
        CHECK (difficulty IN ('easy', 'adaptive', 'ambitious')),
    primary_motivation      text,
    common_blockers         jsonb NOT NULL DEFAULT '[]',
    coaching_notes          jsonb NOT NULL DEFAULT '{}',
    last_context_refresh_at timestamptz,
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER coaching_profiles_set_updated_at
    BEFORE UPDATE ON coaching_profiles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
