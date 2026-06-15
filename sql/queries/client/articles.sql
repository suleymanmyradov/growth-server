-- name: ListArticles :many
SELECT
    a.id, a.title, a.excerpt, a.content, a.read_time_minutes AS read_time, a.image_url, a.author,
    a.published_at, a.created_at, a.updated_at,
    c.id AS category_id, c.name AS category_name, c.slug AS category_slug,
    (SELECT COUNT(*) FROM article_likes al WHERE al.article_id = a.id) AS like_count
FROM articles a
LEFT JOIN categories c ON a.category_id = c.id
ORDER BY a.published_at DESC
LIMIT $1 OFFSET $2;

-- name: ListArticlesWithSaved :many
SELECT
    a.id, a.title, a.excerpt, a.content, a.read_time_minutes AS read_time, a.image_url, a.author,
    a.published_at, a.created_at, a.updated_at,
    c.id AS category_id, c.name AS category_name, c.slug AS category_slug,
    EXISTS(SELECT 1 FROM saved_articles sa WHERE sa.user_id = $3 AND sa.article_id = a.id) AS is_saved,
    (SELECT COUNT(*) FROM article_likes al WHERE al.article_id = a.id) AS like_count,
    EXISTS(SELECT 1 FROM article_likes al2 WHERE al2.user_id = $3 AND al2.article_id = a.id) AS is_liked
FROM articles a
LEFT JOIN categories c ON a.category_id = c.id
ORDER BY a.published_at DESC
LIMIT $1 OFFSET $2;

-- name: ListArticlesByCategorySlug :many
SELECT 
    a.id, a.title, a.excerpt, a.content, a.read_time_minutes AS read_time, a.image_url, a.author, 
    a.published_at, a.created_at, a.updated_at,
    c.id AS category_id, c.name AS category_name, c.slug AS category_slug,
    (SELECT COUNT(*) FROM article_likes al WHERE al.article_id = a.id) AS like_count
FROM articles a
JOIN categories c ON a.category_id = c.id
WHERE c.slug = $1
ORDER BY a.published_at DESC
LIMIT $2 OFFSET $3;

-- name: ListArticlesByCategorySlugWithSaved :many
SELECT 
    a.id, a.title, a.excerpt, a.content, a.read_time_minutes AS read_time, a.image_url, a.author, 
    a.published_at, a.created_at, a.updated_at,
    c.id AS category_id, c.name AS category_name, c.slug AS category_slug,
    EXISTS(SELECT 1 FROM saved_articles sa WHERE sa.user_id = $4 AND sa.article_id = a.id) AS is_saved,
    (SELECT COUNT(*) FROM article_likes al WHERE al.article_id = a.id) AS like_count,
    EXISTS(SELECT 1 FROM article_likes al2 WHERE al2.user_id = $4 AND al2.article_id = a.id) AS is_liked
FROM articles a
JOIN categories c ON a.category_id = c.id
LEFT JOIN saved_articles sa ON sa.article_id = a.id AND sa.user_id = $4
WHERE c.slug = $1
ORDER BY a.published_at DESC
LIMIT $2 OFFSET $3;

-- name: ListArticlesByAuthor :many
SELECT 
    a.id, a.title, a.excerpt, a.content, a.read_time_minutes AS read_time, a.image_url, a.author, 
    a.published_at, a.created_at, a.updated_at,
    c.id AS category_id, c.name AS category_name, c.slug AS category_slug,
    (SELECT COUNT(*) FROM article_likes al WHERE al.article_id = a.id) AS like_count
FROM articles a
LEFT JOIN categories c ON a.category_id = c.id
WHERE a.author = $1
ORDER BY a.published_at DESC
LIMIT $2 OFFSET $3;

-- name: ListArticlesByAuthorWithSaved :many
SELECT 
    a.id, a.title, a.excerpt, a.content, a.read_time_minutes AS read_time, a.image_url, a.author, 
    a.published_at, a.created_at, a.updated_at,
    c.id AS category_id, c.name AS category_name, c.slug AS category_slug,
    EXISTS(SELECT 1 FROM saved_articles sa WHERE sa.user_id = $4 AND sa.article_id = a.id) AS is_saved,
    (SELECT COUNT(*) FROM article_likes al WHERE al.article_id = a.id) AS like_count,
    EXISTS(SELECT 1 FROM article_likes al2 WHERE al2.user_id = $4 AND al2.article_id = a.id) AS is_liked
FROM articles a
LEFT JOIN categories c ON a.category_id = c.id
WHERE a.author = $1
ORDER BY a.published_at DESC
LIMIT $2 OFFSET $3;

-- name: SearchArticles :many
SELECT 
    a.id, a.title, a.excerpt, a.content, a.read_time_minutes AS read_time, a.image_url, a.author, 
    a.published_at, a.created_at, a.updated_at,
    c.id AS category_id, c.name AS category_name, c.slug AS category_slug,
    (SELECT COUNT(*) FROM article_likes al WHERE al.article_id = a.id) AS like_count
FROM articles a
LEFT JOIN categories c ON a.category_id = c.id
WHERE a.search_vector @@ plainto_tsquery('english', $1)
ORDER BY a.published_at DESC
LIMIT $2 OFFSET $3;

-- name: GetArticle :one
SELECT 
    a.id, a.title, a.excerpt, a.content, a.read_time_minutes AS read_time, a.image_url, a.author, 
    a.published_at, a.created_at, a.updated_at,
    c.id AS category_id, c.name AS category_name, c.slug AS category_slug,
    (SELECT COUNT(*) FROM article_likes al WHERE al.article_id = a.id) AS like_count
FROM articles a
LEFT JOIN categories c ON a.category_id = c.id
WHERE a.id = $1;

-- name: GetArticleWithSaved :one
SELECT 
    a.id, a.title, a.excerpt, a.content, a.read_time_minutes AS read_time, a.image_url, a.author, 
    a.published_at, a.created_at, a.updated_at,
    c.id AS category_id, c.name AS category_name, c.slug AS category_slug,
    EXISTS(SELECT 1 FROM saved_articles sa WHERE sa.user_id = $2 AND sa.article_id = a.id) AS is_saved,
    (SELECT COUNT(*) FROM article_likes al WHERE al.article_id = a.id) AS like_count,
    EXISTS(SELECT 1 FROM article_likes al2 WHERE al2.user_id = $2 AND al2.article_id = a.id) AS is_liked
FROM articles a
LEFT JOIN categories c ON a.category_id = c.id
WHERE a.id = $1;

-- name: GetArticleByTitle :one
SELECT 
    a.id, a.title, a.excerpt, a.content, a.read_time_minutes AS read_time, a.image_url, a.author, 
    a.published_at, a.created_at, a.updated_at,
    c.id AS category_id, c.name AS category_name, c.slug AS category_slug,
    (SELECT COUNT(*) FROM article_likes al WHERE al.article_id = a.id) AS like_count
FROM articles a
LEFT JOIN categories c ON a.category_id = c.id
WHERE a.title = $1;

-- name: CreateArticle :one
INSERT INTO articles (title, excerpt, content, category_id, read_time_minutes, image_url, author)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, title, excerpt, content, read_time_minutes AS read_time, image_url, author, published_at, created_at, updated_at;

-- name: UpdateArticle :one
UPDATE articles
SET title = $2, excerpt = $3, content = $4, category_id = $5,
    read_time_minutes = $6, image_url = $7, author = $8
WHERE id = $1
RETURNING id, title, excerpt, content, read_time_minutes AS read_time, image_url, author, published_at, created_at, updated_at;

-- name: DeleteArticle :exec
DELETE FROM articles WHERE id = $1;

-- name: CountArticles :one
SELECT COUNT(*) FROM articles;

-- name: CountArticlesByCategorySlug :one
SELECT COUNT(*) FROM articles a
JOIN categories c ON a.category_id = c.id
WHERE c.slug = $1;


-- name: GetArticlesByIDs :many
-- Bulk lookup for article list views (e.g. resolving saved articles).
-- Uses ANY with a uuid array to avoid N+1 queries.
SELECT
    a.id, a.title, a.excerpt, a.content, a.read_time_minutes AS read_time, a.image_url, a.author,
    a.published_at, a.created_at, a.updated_at,
    c.id AS category_id, c.name AS category_name, c.slug AS category_slug,
    (SELECT COUNT(*) FROM article_likes al WHERE al.article_id = a.id) AS like_count
FROM articles a
LEFT JOIN categories c ON a.category_id = c.id
WHERE a.id = ANY($1::uuid[]);

