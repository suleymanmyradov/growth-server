-- Local read model for user profiles, owned by the client service.
-- Fed by user_profile_updated and user_deleted events from auth.
-- Replaces the cross-service read of the auth-owned users table (V3).
CREATE TABLE user_profiles (
    id          uuid PRIMARY KEY,
    username    varchar NOT NULL DEFAULT '',
    full_name   varchar NOT NULL DEFAULT '',
    bio         text,
    location    varchar,
    website     varchar,
    interests   text[],
    avatar_url  varchar,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER user_profiles_set_updated_at
    BEFORE UPDATE ON user_profiles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
