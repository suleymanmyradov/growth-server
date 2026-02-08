-- name: ListArticles :many
SELECT id, title, excerpt, content, category, read_time, image_url, author, published_at, created_at, updated_at FROM articles
ORDER BY published_at DESC
LIMIT $1 OFFSET $2;

-- name: ListArticlesByCategory :many
SELECT id, title, excerpt, content, category, read_time, image_url, author, published_at, created_at, updated_at FROM articles WHERE category = $1
ORDER BY published_at DESC
LIMIT $2 OFFSET $3;

-- name: ListArticlesByAuthor :many
SELECT id, title, excerpt, content, category, read_time, image_url, author, published_at, created_at, updated_at FROM articles WHERE author = $1
ORDER BY published_at DESC
LIMIT $2 OFFSET $3;

-- name: SearchArticles :many
SELECT id, title, excerpt, content, category, read_time, image_url, author, published_at, created_at, updated_at FROM articles
WHERE search_vector @@ plainto_tsquery('english', $1)
ORDER BY published_at DESC
LIMIT $2 OFFSET $3;

-- name: GetArticle :one
SELECT id, title, excerpt, content, category, read_time, image_url, author, published_at, created_at, updated_at FROM articles WHERE id = $1;

-- name: GetArticleByTitle :one
SELECT id, title, excerpt, content, category, read_time, image_url, author, published_at, created_at, updated_at FROM articles WHERE title = $1;

-- name: CreateArticle :one
INSERT INTO articles (title, excerpt, content, category, read_time, image_url, author)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, title, excerpt, content, category, read_time, image_url, author, published_at, created_at, updated_at;

-- name: UpdateArticle :one
UPDATE articles
SET title = $2, excerpt = $3, content = $4, category = $5,
    read_time = $6, image_url = $7, author = $8, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING id, title, excerpt, content, category, read_time, image_url, author, published_at, created_at, updated_at;

-- name: DeleteArticle :exec
DELETE FROM articles WHERE id = $1;

-- name: CountArticles :one
SELECT COUNT(*) FROM articles;

-- name: CountArticlesByCategory :one
SELECT COUNT(*) FROM articles WHERE category = $1;
