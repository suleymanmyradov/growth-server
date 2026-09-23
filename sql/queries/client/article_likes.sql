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

-- name: ToggleArticleLike :one
-- Atomic flip for the legacy toggle path: a single statement so concurrent
-- toggles can't both read "not liked" and race the insert. Returns the new
-- state (true when the row was inserted by this statement, false when deleted).
WITH del AS (
    DELETE FROM article_likes al
    WHERE al.article_id = $1 AND al.user_id = $2
    RETURNING al.article_id AS deleted_article_id
),
ins AS (
    INSERT INTO article_likes (article_id, user_id)
    SELECT $1::uuid, $2::uuid
    WHERE NOT EXISTS (SELECT 1 FROM del)
    ON CONFLICT (article_id, user_id) DO NOTHING
    RETURNING article_id AS inserted_article_id
)
SELECT EXISTS(SELECT 1 FROM ins) AS liked;
