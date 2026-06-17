-- name: CreateInternalUser :one
INSERT INTO internal_users (email, password_hash, full_name, role)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetInternalUserByEmail :one
SELECT * FROM internal_users WHERE email = $1;

-- name: GetInternalUserByID :one
SELECT * FROM internal_users WHERE id = $1;

-- name: UpdateInternalUserPassword :one
UPDATE internal_users
SET password_hash = $2
WHERE id = $1
RETURNING *;

-- name: UpdateInternalUserProfile :one
UPDATE internal_users
SET full_name = $2
WHERE id = $1
RETURNING *;
