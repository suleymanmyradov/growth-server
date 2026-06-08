-- name: CreateArticleLike :one
INSERT INTO article_likes (article_id, user_id)
VALUES ($1, $2)
ON CONFLICT (article_id, user_id) DO NOTHING
RETURNING id, article_id, user_id, created_at;

-- name: DeleteArticleLike :exec
DELETE FROM article_likes
WHERE article_id = $1 AND user_id = $2;

-- name: CountArticleLikes :one
SELECT COUNT(*) FROM article_likes WHERE article_id = $1;

-- name: IsArticleLikedByUser :one
SELECT EXISTS(SELECT 1 FROM article_likes WHERE article_id = $1 AND user_id = $2) AS is_liked;
