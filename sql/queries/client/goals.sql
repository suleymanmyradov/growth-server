-- name: ListGoals :many
SELECT id, title, description, category, due_date, progress, completed, user_id, created_at, updated_at, status, version
FROM goals
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetGoal :one
SELECT id, title, description, category, due_date, progress, completed, user_id, created_at, updated_at, status, version
FROM goals
WHERE id = $1;

-- name: CreateGoal :one
INSERT INTO goals (title, description, category, due_date, user_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, title, description, category, due_date, progress, completed, user_id, created_at, updated_at, status, version;

-- name: UpdateGoal :one
UPDATE goals
SET title = $2, description = $3, category = $4, due_date = $5, version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND version = $6
RETURNING id, title, description, category, due_date, progress, completed, user_id, created_at, updated_at, status, version;

-- name: DeleteGoal :exec
DELETE FROM goals WHERE id = $1;

-- name: ToggleGoal :one
UPDATE goals
SET status = CASE WHEN status = 'completed' THEN 'active'::goal_status_type ELSE 'completed'::goal_status_type END,
    progress = CASE WHEN status = 'completed' THEN 0 ELSE 100 END,
    version = version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND version = $2
RETURNING id, title, description, category, due_date, progress, completed, user_id, created_at, updated_at, status, version;

-- name: UpdateGoalProgress :one
UPDATE goals
SET progress = $2,
    status = CASE WHEN $2 >= 100 THEN 'completed'::goal_status_type ELSE status END,
    version = version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1 AND version = $3
RETURNING id, title, description, category, due_date, progress, completed, user_id, created_at, updated_at, status, version;

-- name: GetGoalsByIDs :many
-- Bulk lookup for goal list views (e.g. resolving saved goals).
SELECT id, title, description, category, due_date, progress, completed, user_id, created_at, updated_at, status, version
FROM goals
WHERE id = ANY($1::uuid[]);

-- name: CountGoalsByUser :one
SELECT COUNT(*) FROM goals WHERE user_id = $1;

-- name: ListGoalsKeyset :many
-- Keyset pagination: more efficient than OFFSET for deep pages.
-- Pass last_created_at from previous page (or NULL for first page).
SELECT id, title, description, category, due_date, progress, completed, user_id, created_at, updated_at, status, version
FROM goals
WHERE user_id = $1
  AND ($2::timestamptz IS NULL OR created_at < $2)
ORDER BY created_at DESC
LIMIT $3;
