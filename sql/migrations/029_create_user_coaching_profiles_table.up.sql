CREATE TABLE user_coaching_profiles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    accountability_style VARCHAR(20) NOT NULL DEFAULT 'balanced'
        CHECK (accountability_style IN ('gentle', 'balanced', 'strict')),
    preferred_tone VARCHAR(30) NOT NULL DEFAULT 'supportive'
        CHECK (preferred_tone IN ('supportive', 'direct', 'warm', 'practical', 'challenging')),
    difficulty_preference VARCHAR(20) NOT NULL DEFAULT 'adaptive'
        CHECK (difficulty_preference IN ('easy', 'adaptive', 'ambitious')),
    primary_motivation TEXT,
    common_blockers JSONB NOT NULL DEFAULT '[]'::jsonb,
    coaching_notes JSONB NOT NULL DEFAULT '{}'::jsonb,
    last_context_refresh_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id)
);

CREATE INDEX idx_user_coaching_profiles_user_id ON user_coaching_profiles(user_id);

CREATE TRIGGER update_user_coaching_profiles_updated_at
    BEFORE UPDATE ON user_coaching_profiles
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();