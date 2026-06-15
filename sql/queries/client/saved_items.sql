-- Saved items live in three concrete tables (saved_articles, saved_goals,
-- saved_habits); rows are exposed with a uniform (id, item_type, item_id,
-- user_id, created_at) shape.

-- name: ListAllSavedItemsByUser :many
-- Optimized: push LIMIT into each UNION ALL arm so PostgreSQL only sorts at most 3*LIMIT rows
-- instead of all saved items. Any item in the top N overall must be in the top N of its table.
SELECT id, item_type, item_id, user_id, created_at FROM (
    (SELECT sa.id, 'article'::text AS item_type, sa.article_id AS item_id, sa.user_id, sa.created_at FROM saved_articles sa WHERE sa.user_id = $1 ORDER BY sa.created_at DESC LIMIT $2)
    UNION ALL
    (SELECT sg.id, 'goal'::text AS item_type, sg.goal_id AS item_id, sg.user_id, sg.created_at FROM saved_goals sg WHERE sg.user_id = $1 ORDER BY sg.created_at DESC LIMIT $2)
    UNION ALL
    (SELECT sh.id, 'habit'::text AS item_type, sh.habit_id AS item_id, sh.user_id, sh.created_at FROM saved_habits sh WHERE sh.user_id = $1 ORDER BY sh.created_at DESC LIMIT $2)
) combined
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListSavedArticlesByUser :many
SELECT sa.id, 'article'::text AS item_type, sa.article_id AS item_id, sa.user_id, sa.created_at FROM saved_articles sa WHERE sa.user_id = $1 ORDER BY sa.created_at DESC LIMIT $2 OFFSET $3;

-- name: ListSavedGoalsByUser :many
SELECT sg.id, 'goal'::text AS item_type, sg.goal_id AS item_id, sg.user_id, sg.created_at FROM saved_goals sg WHERE sg.user_id = $1 ORDER BY sg.created_at DESC LIMIT $2 OFFSET $3;

-- name: ListSavedHabitsByUser :many
SELECT sh.id, 'habit'::text AS item_type, sh.habit_id AS item_id, sh.user_id, sh.created_at FROM saved_habits sh WHERE sh.user_id = $1 ORDER BY sh.created_at DESC LIMIT $2 OFFSET $3;

-- name: ListSavedArticlesByUserKeyset :many
SELECT sa.id, 'article'::text AS item_type, sa.article_id AS item_id, sa.user_id, sa.created_at FROM saved_articles sa WHERE sa.user_id = $1 AND ($2::timestamptz IS NULL OR sa.created_at < $2) ORDER BY sa.created_at DESC LIMIT $3;

-- name: ListSavedGoalsByUserKeyset :many
SELECT sg.id, 'goal'::text AS item_type, sg.goal_id AS item_id, sg.user_id, sg.created_at FROM saved_goals sg WHERE sg.user_id = $1 AND ($2::timestamptz IS NULL OR sg.created_at < $2) ORDER BY sg.created_at DESC LIMIT $3;

-- name: ListSavedHabitsByUserKeyset :many
SELECT sh.id, 'habit'::text AS item_type, sh.habit_id AS item_id, sh.user_id, sh.created_at FROM saved_habits sh WHERE sh.user_id = $1 AND ($2::timestamptz IS NULL OR sh.created_at < $2) ORDER BY sh.created_at DESC LIMIT $3;

-- name: CreateSavedArticle :one
INSERT INTO saved_articles (article_id, user_id) VALUES ($1, $2) RETURNING id, 'article'::text AS item_type, article_id AS item_id, user_id, created_at;

-- name: CreateSavedGoal :one
INSERT INTO saved_goals (goal_id, user_id) VALUES ($1, $2) RETURNING id, 'goal'::text AS item_type, goal_id AS item_id, user_id, created_at;

-- name: CreateSavedHabit :one
INSERT INTO saved_habits (habit_id, user_id) VALUES ($1, $2) RETURNING id, 'habit'::text AS item_type, habit_id AS item_id, user_id, created_at;

-- name: BatchCreateSavedArticles :copyfrom
INSERT INTO saved_articles (article_id, user_id) VALUES ($1, $2);

-- name: BatchCreateSavedGoals :copyfrom
INSERT INTO saved_goals (goal_id, user_id) VALUES ($1, $2);

-- name: BatchCreateSavedHabits :copyfrom
INSERT INTO saved_habits (habit_id, user_id) VALUES ($1, $2);

-- name: DeleteSavedArticle :exec
DELETE FROM saved_articles sa WHERE sa.user_id = $1 AND sa.article_id = $2;

-- name: DeleteSavedGoal :exec
DELETE FROM saved_goals sg WHERE sg.user_id = $1 AND sg.goal_id = $2;

-- name: DeleteSavedHabit :exec
DELETE FROM saved_habits sh WHERE sh.user_id = $1 AND sh.habit_id = $2;

-- name: IsArticleSaved :one
SELECT EXISTS(SELECT 1 FROM saved_articles sa WHERE sa.user_id = $1 AND sa.article_id = $2);

-- name: IsGoalSaved :one
SELECT EXISTS(SELECT 1 FROM saved_goals sg WHERE sg.user_id = $1 AND sg.goal_id = $2);

-- name: IsHabitSaved :one
SELECT EXISTS(SELECT 1 FROM saved_habits sh WHERE sh.user_id = $1 AND sh.habit_id = $2);

-- name: CountAllSavedItemsByUser :one
SELECT
    (SELECT COUNT(*) FROM saved_articles sa WHERE sa.user_id = $1) +
    (SELECT COUNT(*) FROM saved_goals sg WHERE sg.user_id = $1) +
    (SELECT COUNT(*) FROM saved_habits sh WHERE sh.user_id = $1);

-- name: CountSavedArticlesByUser :one
SELECT COUNT(*) FROM saved_articles WHERE user_id = $1;

-- name: CountSavedGoalsByUser :one
SELECT COUNT(*) FROM saved_goals WHERE user_id = $1;

-- name: CountSavedHabitsByUser :one
SELECT COUNT(*) FROM saved_habits WHERE user_id = $1;
