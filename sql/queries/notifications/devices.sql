-- Device installation / push token queries.
-- Owned exclusively by the notifications service. See migration 042 and
-- docs/push-notifications-design.md.

-- name: UpsertDevice :one
-- Idempotent register/update: insert a new device or update the push token +
-- metadata for an existing (installation_id, user_id) pair. Token rotation is
-- handled by the UPDATE branch. enabled is reset to true on re-registration.
INSERT INTO notification_devices (
    user_id, installation_id, provider, push_token, platform, app_id,
    environment, app_version, os_version, locale, timezone, enabled, last_seen_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, true, now())
ON CONFLICT (installation_id, user_id) DO UPDATE
SET
    provider     = EXCLUDED.provider,
    push_token   = EXCLUDED.push_token,
    platform     = EXCLUDED.platform,
    app_id       = EXCLUDED.app_id,
    environment  = EXCLUDED.environment,
    app_version  = EXCLUDED.app_version,
    os_version   = EXCLUDED.os_version,
    locale       = EXCLUDED.locale,
    timezone     = EXCLUDED.timezone,
    enabled      = true,
    last_seen_at = now()
RETURNING *;

-- name: DeleteDevice :exec
-- Unregister a device by installation_id + user_id. Used on logout and on
-- explicit unregister. The user_id scoping prevents a user from deleting
-- another user's device registration.
DELETE FROM notification_devices
WHERE installation_id = $1 AND user_id = $2;

-- name: ListActiveDevicesByUser :many
-- All enabled devices for a user, used to fan out push notifications.
SELECT * FROM notification_devices
WHERE user_id = $1 AND enabled = true
ORDER BY created_at DESC;

-- name: DisableDeviceByToken :exec
-- Mark a device disabled when the push provider reports the token as invalid
-- (e.g. Expo receipt API returns a DeviceNotRegistered error). The token is
-- the only stable identifier the provider returns, so we disable by token.
UPDATE notification_devices
SET enabled = false
WHERE push_token = $1;

-- name: UpdateDeviceLastSeen :exec
-- Bump last_seen_at on a registration/heartbeat so we can age out stale
-- installations later.
UPDATE notification_devices
SET last_seen_at = now()
WHERE installation_id = $1 AND user_id = $2;
