-- name: GetUserPreferences :one
SELECT user_id, theme, language, timezone, check_in_time, onboarding_completed, created_at, updated_at
FROM user_preferences
WHERE user_id = $1;

-- name: CreateUserPreferences :one
INSERT INTO user_preferences (
    theme, language, timezone, user_id
) VALUES ($1, $2, $3, $4)
RETURNING user_id, theme, language, timezone, check_in_time, onboarding_completed, created_at, updated_at;

-- name: UpdateUserPreferences :one
INSERT INTO user_preferences (user_id, theme, language, timezone)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id) DO UPDATE
SET theme = EXCLUDED.theme, language = EXCLUDED.language, timezone = EXCLUDED.timezone
RETURNING user_id, theme, language, timezone, check_in_time, onboarding_completed, created_at, updated_at;

-- name: UpdateOnboardingCompleted :one
-- The onboarding_completed flag is a one-way operation: once true, a general
-- settings update (e.g. changing check-in time or accountability style) must
-- never reset it to false. The protobuf bool field defaults to false when
-- omitted, so the CASE expression preserves the existing value when the input
-- is false and only flips to true when explicitly requested.
INSERT INTO user_preferences (user_id, check_in_time, onboarding_completed)
VALUES (sqlc.arg(user_id), COALESCE(sqlc.arg(check_in_time)::time, '09:00'::time), sqlc.arg(onboarding_completed))
ON CONFLICT (user_id) DO UPDATE
SET check_in_time = COALESCE(sqlc.arg(check_in_time)::time, user_preferences.check_in_time),
    onboarding_completed = CASE WHEN sqlc.arg(onboarding_completed) THEN true ELSE user_preferences.onboarding_completed END
RETURNING user_id, theme, language, timezone, check_in_time, onboarding_completed, created_at, updated_at;

-- name: DeleteUserPreferences :exec
DELETE FROM user_preferences WHERE user_id = $1;
