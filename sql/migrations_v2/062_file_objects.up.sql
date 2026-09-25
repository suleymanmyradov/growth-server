-- File-object registry for the filemanager service.
--
-- Uploads land in MinIO under random keys (folder/uuid.ext), so without this
-- table there is no way to answer "which objects belong to user X" — account
-- deletion could not find a user's files, and DeleteFile could not verify the
-- caller owns the key it is deleting. Every stored object gets one row here.
--
-- owner_user_id is nullable: admin/service uploads (e.g. article images) have
-- no end-user owner. NULL-owned objects are invisible to user-deletion
-- cleanup and can never satisfy a user-scoped DeleteFile ownership check.
-- Per the schema design rules there is deliberately no FK to users — the
-- reference crosses a service boundary, so cleanup rides the user_deleted
-- event instead.
--
-- expires_at drives retention sweeps (e.g. exports/ objects are deleted 24h
-- after upload). NULL means "keep until explicit delete".

CREATE TABLE file_objects (
    bucket        text        NOT NULL,
    object_key    text        NOT NULL,
    owner_user_id uuid,
    folder        text        NOT NULL,
    content_type  text        NOT NULL,
    size_bytes    bigint      NOT NULL,
    expires_at    timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (bucket, object_key)
);

-- "All objects owned by user X" — account-deletion fan-out.
CREATE INDEX file_objects_owner_idx ON file_objects (owner_user_id)
    WHERE owner_user_id IS NOT NULL;

-- "Objects past their retention deadline" — expiry sweeper.
CREATE INDEX file_objects_expires_idx ON file_objects (expires_at)
    WHERE expires_at IS NOT NULL;
