-- name: GetOAuthAccount :one
SELECT id, user_id, provider, provider_uid, email, created_at
FROM user_oauth_accounts
WHERE provider = $1 AND provider_uid = $2;

-- name: CreateOAuthAccount :one
INSERT INTO user_oauth_accounts (user_id, provider, provider_uid, email)
VALUES ($1, $2, $3, $4)
RETURNING id, user_id, provider, provider_uid, email, created_at;
