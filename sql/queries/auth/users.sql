-- name: CreateUser :one
INSERT INTO users (username, email, password_hash, full_name)
VALUES ($1, $2, $3, $4)
RETURNING id, username, email, password_hash, full_name, created_at, updated_at;

-- name: GetUserByEmail :one
SELECT id, username, email, password_hash, full_name, created_at, updated_at
FROM users
WHERE email = $1;

-- name: GetUserByID :one
SELECT id, username, email, password_hash, full_name, created_at, updated_at
FROM users
WHERE id = $1;

-- name: GetUserByUsername :one
SELECT id, username, email, password_hash, full_name, created_at, updated_at
FROM users
WHERE username = $1;

-- name: UpdateUserPassword :one
UPDATE users
SET password_hash = $2, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING id, username, email, password_hash, full_name, created_at, updated_at;

-- name: UpdateUserFullName :one
UPDATE users
SET full_name = $2, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING id, username, email, password_hash, full_name, created_at, updated_at;
