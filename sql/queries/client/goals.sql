-- name: ListGoals :many
SELECT id, title, description, category, due_date, progress, completed, user_id, created_at, updated_at
FROM goals
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetGoal :one
SELECT id, title, description, category, due_date, progress, completed, user_id, created_at, updated_at
FROM goals
WHERE id = $1;

-- name: CreateGoal :one
INSERT INTO goals (title, description, category, due_date, user_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, title, description, category, due_date, progress, completed, user_id, created_at, updated_at;

-- name: UpdateGoal :one
UPDATE goals
SET title = $2, description = $3, category = $4, due_date = $5, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING id, title, description, category, due_date, progress, completed, user_id, created_at, updated_at;

-- name: DeleteGoal :exec
DELETE FROM goals WHERE id = $1;

-- name: ToggleGoal :one
UPDATE goals
SET completed = NOT completed,
    progress = CASE WHEN NOT completed THEN 100 ELSE 0 END,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING id, title, description, category, due_date, progress, completed, user_id, created_at, updated_at;

-- name: UpdateGoalProgress :one
UPDATE goals
SET progress = $2,
    completed = CASE WHEN $2 >= 100 THEN true ELSE completed END,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING id, title, description, category, due_date, progress, completed, user_id, created_at, updated_at;

-- name: CountGoalsByUser :one
SELECT COUNT(*) FROM goals WHERE user_id = $1;
