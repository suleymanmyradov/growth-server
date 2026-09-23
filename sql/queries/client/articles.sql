-- name: ListArticles :many
SELECT
    a.id, a.title, a.excerpt, a.content, a.read_time_minutes AS read_time, a.image_url, a.author,
    a.published_at, a.created_at, a.updated_at, a.status,
    c.id AS category_id, c.name AS category_name, c.slug AS category_slug,
    (SELECT COUNT(*) FROM article_likes al WHERE al.article_id = a.id) AS like_count
FROM articles a
LEFT JOIN categories c ON a.category_id = c.id
WHERE (sqlc.arg(status)::text = '' OR a.status = sqlc.arg(status)::text)
ORDER BY a.published_at DESC
LIMIT $1 OFFSET $2;

-- name: ListArticlesWithSaved :many
SELECT
    a.id, a.title, a.excerpt, a.content, a.read_time_minutes AS read_time, a.image_url, a.author,
    a.published_at, a.created_at, a.updated_at, a.status,
    c.id AS category_id, c.name AS category_name, c.slug AS category_slug,
    EXISTS(SELECT 1 FROM saved_articles sa WHERE sa.user_id = $3 AND sa.article_id = a.id) AS is_saved,
    (SELECT COUNT(*) FROM article_likes al WHERE al.article_id = a.id) AS like_count,
    EXISTS(SELECT 1 FROM article_likes al2 WHERE al2.user_id = $3 AND al2.article_id = a.id) AS is_liked
FROM articles a
LEFT JOIN categories c ON a.category_id = c.id
WHERE (sqlc.arg(status)::text = '' OR a.status = sqlc.arg(status)::text)
ORDER BY a.published_at DESC
LIMIT $1 OFFSET $2;

-- name: ListArticlesByCategorySlug :many
SELECT
    a.id, a.title, a.excerpt, a.content, a.read_time_minutes AS read_time, a.image_url, a.author,
    a.published_at, a.created_at, a.updated_at, a.status,
    c.id AS category_id, c.name AS category_name, c.slug AS category_slug,
    (SELECT COUNT(*) FROM article_likes al WHERE al.article_id = a.id) AS like_count
FROM articles a
JOIN categories c ON a.category_id = c.id
WHERE c.slug = $1
  AND (sqlc.arg(status)::text = '' OR a.status = sqlc.arg(status)::text)
ORDER BY a.published_at DESC
LIMIT $2 OFFSET $3;

-- name: ListArticlesByCategorySlugWithSaved :many
SELECT
    a.id, a.title, a.excerpt, a.content, a.read_time_minutes AS read_time, a.image_url, a.author,
    a.published_at, a.created_at, a.updated_at, a.status,
    c.id AS category_id, c.name AS category_name, c.slug AS category_slug,
    EXISTS(SELECT 1 FROM saved_articles sa WHERE sa.user_id = $4 AND sa.article_id = a.id) AS is_saved,
    (SELECT COUNT(*) FROM article_likes al WHERE al.article_id = a.id) AS like_count,
    EXISTS(SELECT 1 FROM article_likes al2 WHERE al2.user_id = $4 AND al2.article_id = a.id) AS is_liked
FROM articles a
JOIN categories c ON a.category_id = c.id
LEFT JOIN saved_articles sa ON sa.article_id = a.id AND sa.user_id = $4
WHERE c.slug = $1
  AND (sqlc.arg(status)::text = '' OR a.status = sqlc.arg(status)::text)
ORDER BY a.published_at DESC
LIMIT $2 OFFSET $3;

-- name: ListArticlesByAuthor :many
SELECT
    a.id, a.title, a.excerpt, a.content, a.read_time_minutes AS read_time, a.image_url, a.author,
    a.published_at, a.created_at, a.updated_at, a.status,
    c.id AS category_id, c.name AS category_name, c.slug AS category_slug,
    (SELECT COUNT(*) FROM article_likes al WHERE al.article_id = a.id) AS like_count
FROM articles a
LEFT JOIN categories c ON a.category_id = c.id
WHERE a.author = $1
  AND (sqlc.arg(status)::text = '' OR a.status = sqlc.arg(status)::text)
ORDER BY a.published_at DESC
LIMIT $2 OFFSET $3;

-- name: ListArticlesByAuthorWithSaved :many
SELECT
    a.id, a.title, a.excerpt, a.content, a.read_time_minutes AS read_time, a.image_url, a.author,
    a.published_at, a.created_at, a.updated_at, a.status,
    c.id AS category_id, c.name AS category_name, c.slug AS category_slug,
    EXISTS(SELECT 1 FROM saved_articles sa WHERE sa.user_id = $4 AND sa.article_id = a.id) AS is_saved,
    (SELECT COUNT(*) FROM article_likes al WHERE al.article_id = a.id) AS like_count,
    EXISTS(SELECT 1 FROM article_likes al2 WHERE al2.user_id = $4 AND al2.article_id = a.id) AS is_liked
FROM articles a
LEFT JOIN categories c ON a.category_id = c.id
WHERE a.author = $1
  AND (sqlc.arg(status)::text = '' OR a.status = sqlc.arg(status)::text)
ORDER BY a.published_at DESC
LIMIT $2 OFFSET $3;

-- name: GetArticle :one
SELECT
    a.id, a.title, a.excerpt, a.content, a.read_time_minutes AS read_time, a.image_url, a.author,
    a.published_at, a.created_at, a.updated_at, a.status,
    c.id AS category_id, c.name AS category_name, c.slug AS category_slug,
    (SELECT COUNT(*) FROM article_likes al WHERE al.article_id = a.id) AS like_count
FROM articles a
LEFT JOIN categories c ON a.category_id = c.id
WHERE a.id = $1
  AND (sqlc.arg(status)::text = '' OR a.status = sqlc.arg(status)::text);

-- name: GetArticleWithSaved :one
SELECT
    a.id, a.title, a.excerpt, a.content, a.read_time_minutes AS read_time, a.image_url, a.author,
    a.published_at, a.created_at, a.updated_at, a.status,
    c.id AS category_id, c.name AS category_name, c.slug AS category_slug,
    EXISTS(SELECT 1 FROM saved_articles sa WHERE sa.user_id = $2 AND sa.article_id = a.id) AS is_saved,
    (SELECT COUNT(*) FROM article_likes al WHERE al.article_id = a.id) AS like_count,
    EXISTS(SELECT 1 FROM article_likes al2 WHERE al2.user_id = $2 AND al2.article_id = a.id) AS is_liked
FROM articles a
LEFT JOIN categories c ON a.category_id = c.id
WHERE a.id = $1
  AND (sqlc.arg(status)::text = '' OR a.status = sqlc.arg(status)::text);

-- name: GetArticleByTitle :one
SELECT
    a.id, a.title, a.excerpt, a.content, a.read_time_minutes AS read_time, a.image_url, a.author,
    a.published_at, a.created_at, a.updated_at, a.status,
    c.id AS category_id, c.name AS category_name, c.slug AS category_slug,
    (SELECT COUNT(*) FROM article_likes al WHERE al.article_id = a.id) AS like_count
FROM articles a
LEFT JOIN categories c ON a.category_id = c.id
WHERE a.title = $1;

-- name: GetFeaturedArticle :one
-- Returns the most "popular" published article from the last 30 days, scored
-- by a weighted sum of likes (x3), saves (x2) and shares (x1). Falls back to
-- most recent when engagement counts tie or are all zero.
SELECT
    a.id, a.title, a.excerpt, a.content, a.read_time_minutes AS read_time, a.image_url, a.author,
    a.published_at, a.created_at, a.updated_at, a.status,
    c.id AS category_id, c.name AS category_name, c.slug AS category_slug,
    (SELECT COUNT(*) FROM article_likes al WHERE al.article_id = a.id) AS like_count
FROM articles a
LEFT JOIN categories c ON a.category_id = c.id
WHERE a.status = 'published'
  AND a.published_at >= now() - interval '30 days'
ORDER BY (
    (SELECT COUNT(*) FROM article_likes al WHERE al.article_id = a.id) * 3
    + (SELECT COUNT(*) FROM saved_articles sa WHERE sa.article_id = a.id) * 2
    + (SELECT COUNT(*) FROM article_shares ash WHERE ash.article_id = a.id)
) DESC, a.published_at DESC
LIMIT 1;

-- name: CreateArticle :one
INSERT INTO articles (title, excerpt, content, category_id, read_time_minutes, image_url, author, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, sqlc.arg(status))
RETURNING id, title, excerpt, content, read_time_minutes AS read_time, image_url, author, status, published_at, created_at, updated_at;

-- name: UpdateArticle :one
-- Empty status means "keep the current status" so callers that don't manage
-- the draft/published lifecycle can't silently re-publish a draft. The same
-- convention applies to every other column: empty text / NULL / 0 means
-- "not provided" and preserves the stored value, because the admin PUT is a
-- partial update, not a full replace.
UPDATE articles
SET title = COALESCE(NULLIF(sqlc.arg(title)::text, ''), articles.title),
    excerpt = COALESCE(sqlc.narg(excerpt), articles.excerpt),
    content = COALESCE(NULLIF(sqlc.arg(content)::text, ''), articles.content),
    category_id = COALESCE(sqlc.narg(category_id), articles.category_id),
    read_time_minutes = CASE WHEN sqlc.arg(read_time_minutes)::int = 0 THEN articles.read_time_minutes ELSE sqlc.arg(read_time_minutes)::int END,
    image_url = COALESCE(sqlc.narg(image_url), articles.image_url),
    author = COALESCE(NULLIF(sqlc.arg(author)::text, ''), articles.author),
    status = CASE WHEN sqlc.arg(status)::text = '' THEN articles.status ELSE sqlc.arg(status)::text END
WHERE id = sqlc.arg(id)
RETURNING id, title, excerpt, content, category_id, read_time_minutes AS read_time, image_url, author, status, published_at, created_at, updated_at;

-- name: DeleteArticle :exec
DELETE FROM articles WHERE id = $1;

-- name: CountArticles :one
SELECT COUNT(*) FROM articles
WHERE (sqlc.arg(status)::text = '' OR status = sqlc.arg(status)::text);

-- name: CountArticlesByCategorySlug :one
SELECT COUNT(*) FROM articles a
JOIN categories c ON a.category_id = c.id
WHERE c.slug = $1
  AND (sqlc.arg(status)::text = '' OR a.status = sqlc.arg(status)::text);

-- name: CountArticlesByCategoryID :one
SELECT COUNT(*) FROM articles WHERE category_id = $1;


-- name: GetArticlesByIDs :many
-- Bulk lookup for article list views (e.g. resolving saved articles).
-- Uses ANY with a uuid array to avoid N+1 queries.
SELECT
    a.id, a.title, a.excerpt, a.content, a.read_time_minutes AS read_time, a.image_url, a.author,
    a.published_at, a.created_at, a.updated_at, a.status,
    c.id AS category_id, c.name AS category_name, c.slug AS category_slug,
    (SELECT COUNT(*) FROM article_likes al WHERE al.article_id = a.id) AS like_count
FROM articles a
LEFT JOIN categories c ON a.category_id = c.id
WHERE a.id = ANY($1::uuid[]);

-- name: UpsertTags :many
INSERT INTO tags (name, slug)
SELECT unnest($1::text[]), unnest($2::text[])
ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
RETURNING id, name, slug;

-- name: DeleteArticleTags :exec
DELETE FROM article_tags WHERE article_id = $1;

-- name: LinkArticleTags :exec
INSERT INTO article_tags (article_id, tag_id)
SELECT $1, id FROM tags WHERE name = ANY($2::varchar[])
ON CONFLICT DO NOTHING;

-- name: GetTagsByArticleIDs :many
SELECT at.article_id, t.name, t.slug
FROM article_tags at
JOIN tags t ON at.tag_id = t.id
WHERE at.article_id = ANY($1::uuid[])
ORDER BY at.article_id, t.name;

-- name: ListTags :many
SELECT id, name, slug
FROM tags
ORDER BY name;
