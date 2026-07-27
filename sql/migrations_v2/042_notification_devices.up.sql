-- Device installation / push token table owned by the notifications service.
-- Stores per-installation Expo push tokens (or direct FCM/APNs tokens) so the
-- notifications service can deliver push notifications.
--
-- No FK to users(id) — users is owned by the auth service and cross-service FKs
-- are forbidden (see backend AGENTS.md "Data ownership"). Cascade on user
-- deletion is handled by the user_deleted event consumer via
-- DeleteDevicesByUser in sql/queries/notifications/user_cleanup.sql.
--
-- Key design decisions (see docs/push-notifications-design.md):
--   - One user can have multiple installations (devices).
--   - installation_id is the app-generated UUID from expo-secure-store, not a
--     hardware fingerprint.
--   - provider defaults to 'expo' (Expo Push Service abstracts APNs/FCM).
--   - Tokens can rotate; registration is an upsert on (installation_id, user_id).
--   - Multiple users can share a physical device over time (different
--     installation_ids).

CREATE TABLE notification_devices (
    id              uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id         uuid NOT NULL,
    installation_id varchar(255) NOT NULL,
    provider        text NOT NULL CHECK (provider IN ('expo', 'fcm', 'apns')),
    push_token      text NOT NULL,
    platform        text NOT NULL CHECK (platform IN ('ios', 'android')),
    app_id          varchar(255),
    environment     text NOT NULL CHECK (environment IN ('development', 'preview', 'production')),
    app_version     varchar(50),
    os_version      varchar(50),
    locale          varchar(20),
    timezone        varchar(50),
    enabled         boolean NOT NULL DEFAULT true,
    last_seen_at    timestamptz NOT NULL DEFAULT now(),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (installation_id, user_id)
);

CREATE INDEX idx_notification_devices_user ON notification_devices (user_id, enabled);
CREATE INDEX idx_notification_devices_token ON notification_devices (push_token);

CREATE TRIGGER notification_devices_set_updated_at
    BEFORE UPDATE ON notification_devices
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
