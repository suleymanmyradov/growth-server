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
UPDATE user_preferences
SET theme = $2, language = $3, timezone = $4
WHERE user_id = $1
RETURNING user_id, theme, language, timezone, check_in_time, onboarding_completed, created_at, updated_at;

-- name: UpdateOnboardingCompleted :one
UPDATE user_preferences
SET check_in_time = $2,
    onboarding_completed = $3
WHERE user_id = $1
RETURNING user_id, theme, language, timezone, check_in_time, onboarding_completed, created_at, updated_at;

-- name: DeleteUserPreferences :exec
DELETE FROM user_preferences WHERE user_id = $1;
