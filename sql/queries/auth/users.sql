-- Column order in all RETURNING/SELECT clauses matches the `users` table
-- definition so sqlc reuses the db.User model struct (avoids per-query Row
-- types). Order: id, username, email, password_hash, full_name, bio, location,
-- website, interests, avatar_url, created_at, updated_at, email_verified.

-- name: CreateUser :one
INSERT INTO users (username, email, password_hash, full_name, email_verified)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, username, email, password_hash, full_name, bio, location, website, interests, avatar_url, created_at, updated_at, email_verified;

-- name: CreateUserOAuth :one
-- Creates a user with no local password (OAuth-only). email_verified is taken
-- from the provider's verified claim.
INSERT INTO users (username, email, password_hash, full_name, email_verified)
VALUES ($1, $2, NULL, $3, $4)
RETURNING id, username, email, password_hash, full_name, bio, location, website, interests, avatar_url, created_at, updated_at, email_verified;

-- name: GetUserByEmail :one
SELECT id, username, email, password_hash, full_name, bio, location, website, interests, avatar_url, created_at, updated_at, email_verified
FROM users
WHERE email = $1;

-- name: GetUserByID :one
SELECT id, username, email, password_hash, full_name, bio, location, website, interests, avatar_url, created_at, updated_at, email_verified
FROM users
WHERE id = $1;

-- name: GetUserByUsername :one
SELECT id, username, email, password_hash, full_name, bio, location, website, interests, avatar_url, created_at, updated_at, email_verified
FROM users
WHERE username = $1;

-- name: UpdateUserPassword :one
UPDATE users
SET password_hash = $2
WHERE id = $1
RETURNING id, username, email, password_hash, full_name, bio, location, website, interests, avatar_url, created_at, updated_at, email_verified;

-- name: UpdateUserFullName :one
UPDATE users
SET full_name = $2
WHERE id = $1
RETURNING id, username, email, password_hash, full_name, bio, location, website, interests, avatar_url, created_at, updated_at, email_verified;

-- name: SetEmailVerified :one
UPDATE users
SET email_verified = true
WHERE id = $1
RETURNING id, username, email, password_hash, full_name, bio, location, website, interests, avatar_url, created_at, updated_at, email_verified;

-- name: UpdateUserProfile :one
UPDATE users
SET bio        = $2,
    location   = $3,
    website    = $4,
    interests  = $5,
    avatar_url = $6
WHERE id = $1
RETURNING id, username, email, password_hash, full_name, bio, location, website, interests, avatar_url, created_at, updated_at, email_verified;
