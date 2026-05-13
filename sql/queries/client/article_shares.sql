-- name: ListArticleShares :many
SELECT id, article_id, user_id, platform, created_at FROM article_shares
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListArticleSharesByArticle :many
SELECT id, article_id, user_id, platform, created_at FROM article_shares WHERE article_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListArticleSharesByUser :many
SELECT id, article_id, user_id, platform, created_at FROM article_shares WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetArticleShare :one
SELECT id, article_id, user_id, platform, created_at FROM article_shares WHERE id = $1;

-- name: GetArticleShareByUserAndArticle :one
SELECT id, article_id, user_id, platform, created_at FROM article_shares WHERE user_id = $1 AND article_id = $2;

-- name: CreateArticleShare :one
INSERT INTO article_shares (article_id, user_id, platform)
VALUES ($1, $2, $3)
RETURNING id, article_id, user_id, platform, created_at;

-- name: DeleteArticleShare :exec
DELETE FROM article_shares WHERE id = $1;

-- name: DeleteArticleShareByUserAndArticle :exec
DELETE FROM article_shares WHERE user_id = $1 AND article_id = $2;

-- name: CountArticleShares :one
SELECT COUNT(*) FROM article_shares;

-- name: CountArticleSharesByArticle :one
SELECT COUNT(*) FROM article_shares WHERE article_id = $1;

-- name: CountArticleSharesByUser :one
SELECT COUNT(*) FROM article_shares WHERE user_id = $1;
