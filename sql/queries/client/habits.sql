-- name: ListHabits :many
SELECT id, name, description, streak, completed, category, user_id, created_at, updated_at, version
FROM habits
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetHabit :one
SELECT id, name, description, streak, completed, category, user_id, created_at, updated_at, version
FROM habits
WHERE id = $1;

-- name: CreateHabit :one
INSERT INTO habits (name, description, category, user_id)
VALUES ($1, $2, $3, $4)
RETURNING id, name, description, streak, completed, category, user_id, created_at, updated_at, version;

-- name: UpdateHabit :one
UPDATE habits
SET name = $2, description = $3, category = $4, version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND version = $5
RETURNING id, name, description, streak, completed, category, user_id, created_at, updated_at, version;

-- name: UpdateHabitStreak :one
UPDATE habits
SET streak = $2, version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND version = $3
RETURNING id, name, description, streak, completed, category, user_id, created_at, updated_at, version;

-- name: DeleteHabit :exec
DELETE FROM habits WHERE id = $1;

-- name: ToggleHabit :one
UPDATE habits
SET completed = NOT completed,
    streak = CASE WHEN NOT completed THEN streak + 1 ELSE GREATEST(streak - 1, 0) END,
    version = version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND version = $2
RETURNING id, name, description, streak, completed, category, user_id, created_at, updated_at, version;

-- name: MarkHabitCompleted :one
UPDATE habits
SET completed = true,
    streak = CASE WHEN NOT completed THEN streak + 1 ELSE streak END,
    version = version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND version = $2
RETURNING id, name, description, streak, completed, category, user_id, created_at, updated_at, version;

-- name: ResetTodayHabits :execrows
UPDATE habits
SET completed = false, updated_at = CURRENT_TIMESTAMP
WHERE user_id = $1 AND completed = true;

-- name: GetHabitsByIDs :many
-- Bulk lookup for habit list views (e.g. resolving saved habits).
SELECT id, name, description, streak, completed, category, user_id, created_at, updated_at, version
FROM habits
WHERE id = ANY($1::uuid[]);

-- name: CountHabitsByUser :one
SELECT COUNT(*) FROM habits WHERE user_id = $1;

-- name: ListHabitsKeyset :many
-- Keyset pagination: more efficient than OFFSET for deep pages.
-- Pass last_created_at from previous page (or NULL for first page).
SELECT id, name, description, streak, completed, category, user_id, created_at, updated_at, version
FROM habits
WHERE user_id = $1
  AND ($2::timestamptz IS NULL OR created_at < $2)
ORDER BY created_at DESC
LIMIT $3;
