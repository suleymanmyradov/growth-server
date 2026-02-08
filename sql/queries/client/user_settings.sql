-- name: GetUserSettings :one
SELECT id, theme, language, timezone, email_notifications, push_notifications, habit_reminders, goal_reminders, user_id, created_at, updated_at FROM user_settings WHERE user_id = $1;

-- name: GetUserSettingsByID :one
SELECT id, theme, language, timezone, email_notifications, push_notifications, habit_reminders, goal_reminders, user_id, created_at, updated_at FROM user_settings WHERE id = $1;

-- name: CreateUserSettings :one
INSERT INTO user_settings (
    theme, language, timezone, email_notifications, push_notifications,
    habit_reminders, goal_reminders, user_id
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, theme, language, timezone, email_notifications, push_notifications, habit_reminders, goal_reminders, user_id, created_at, updated_at;

-- name: UpdateUserSettings :one
UPDATE user_settings
SET theme = $2, language = $3, timezone = $4, email_notifications = $5,
    push_notifications = $6, habit_reminders = $7, goal_reminders = $8, updated_at = CURRENT_TIMESTAMP
WHERE user_id = $1
RETURNING id, theme, language, timezone, email_notifications, push_notifications, habit_reminders, goal_reminders, user_id, created_at, updated_at;

-- name: DeleteUserSettings :exec
DELETE FROM user_settings WHERE user_id = $1;

-- name: CountUserSettings :one
SELECT COUNT(*) FROM user_settings;
