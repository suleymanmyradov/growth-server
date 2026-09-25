-- name: CreateFileObject :exec
-- Registry row written after the object is stored. ON CONFLICT overwrites the
-- stale metadata for the same (bucket, key) — keys are random UUIDs so a
-- collision is practically impossible, but a retried insert must not fail.
INSERT INTO file_objects (bucket, object_key, owner_user_id, folder, content_type, size_bytes, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (bucket, object_key) DO UPDATE SET
    owner_user_id = EXCLUDED.owner_user_id,
    folder        = EXCLUDED.folder,
    content_type  = EXCLUDED.content_type,
    size_bytes    = EXCLUDED.size_bytes,
    expires_at    = EXCLUDED.expires_at;

-- name: GetFileObject :one
-- Ownership lookup for DeleteFile authorization.
SELECT bucket, object_key, owner_user_id, folder, content_type, size_bytes, expires_at, created_at
FROM file_objects
WHERE bucket = $1 AND object_key = $2;

-- name: DeleteFileObject :exec
-- Removes the registry row after the object is gone (or confirmed missing).
DELETE FROM file_objects
WHERE bucket = $1 AND object_key = $2;

-- name: ListFileObjectsByOwner :many
-- Every object owned by a user — the user_deleted fan-out. Ordered for
-- deterministic processing; callers delete objects then rows.
SELECT bucket, object_key, owner_user_id, folder, content_type, size_bytes, expires_at, created_at
FROM file_objects
WHERE owner_user_id = $1
ORDER BY created_at;

-- name: ListExpiredFileObjects :many
-- Objects past their retention deadline for the cleanup sweeper.
SELECT bucket, object_key, owner_user_id, folder, content_type, size_bytes, expires_at, created_at
FROM file_objects
WHERE expires_at IS NOT NULL AND expires_at <= now()
ORDER BY expires_at
LIMIT $1;
