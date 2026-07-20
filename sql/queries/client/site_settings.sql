-- name: GetSiteSetting :one
SELECT key, value, created_at, updated_at
FROM site_settings
WHERE key = $1;

-- name: ListSiteSettings :many
SELECT key, value, created_at, updated_at
FROM site_settings
WHERE key = ANY($1::text[])
ORDER BY key;

-- name: ListAllSiteSettings :many
SELECT key, value, created_at, updated_at
FROM site_settings
ORDER BY key;

-- name: UpsertSiteSetting :one
INSERT INTO site_settings (key, value)
VALUES ($1, $2)
ON CONFLICT (key) DO UPDATE
    SET value = EXCLUDED.value,
        updated_at = now()
RETURNING key, value, created_at, updated_at;

-- name: DeleteSiteSetting :exec
DELETE FROM site_settings WHERE key = $1;
