-- name: ListHabits :many
SELECT id, name, description, streak, completed, category, user_id, created_at, updated_at
FROM habits
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetHabit :one
SELECT id, name, description, streak, completed, category, user_id, created_at, updated_at
FROM habits
WHERE id = $1;

-- name: CreateHabit :one
INSERT INTO habits (name, description, category, user_id)
VALUES ($1, $2, $3, $4)
RETURNING id, name, description, streak, completed, category, user_id, created_at, updated_at;

-- name: UpdateHabit :one
UPDATE habits
SET name = $2, description = $3, category = $4, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING id, name, description, streak, completed, category, user_id, created_at, updated_at;

-- name: UpdateHabitStreak :one
UPDATE habits
SET streak = $2, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING id, name, description, streak, completed, category, user_id, created_at, updated_at;

-- name: DeleteHabit :exec
DELETE FROM habits WHERE id = $1;

-- name: ToggleHabit :one
UPDATE habits
SET completed = NOT completed,
    streak = CASE WHEN NOT completed THEN streak + 1 ELSE streak END,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING id, name, description, streak, completed, category, user_id, created_at, updated_at;

-- name: ResetTodayHabits :execrows
UPDATE habits
SET completed = false, updated_at = CURRENT_TIMESTAMP
WHERE user_id = $1 AND completed = true;

-- name: CountHabitsByUser :one
SELECT COUNT(*) FROM habits WHERE user_id = $1;
