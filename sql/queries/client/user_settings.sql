-- name: GetUserSettings :one
SELECT * FROM user_settings WHERE user_id = $1;

-- name: CreateUserSettings :one
INSERT INTO user_settings (
    theme, language, timezone, email_notifications, push_notifications,
    habit_reminders, goal_reminders, user_id
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateUserSettings :one
UPDATE user_settings
SET theme = $2, language = $3, timezone = $4, email_notifications = $5,
    push_notifications = $6, habit_reminders = $7, goal_reminders = $8
WHERE user_id = $1
RETURNING *;

-- name: UpdateOnboardingSettings :one
UPDATE user_settings
SET accountability_style = $2,
    check_in_time = $3,
    onboarding_completed = $4
WHERE user_id = $1
RETURNING *;

-- name: DeleteUserSettings :exec
DELETE FROM user_settings WHERE user_id = $1;
