-- Key/value store for admin-managed site-wide settings (explore header,
-- community card links, tab order, etc.). Values are JSONB so each key can
-- carry its own structured payload.
CREATE TABLE site_settings (
    key        varchar(50) PRIMARY KEY,
    value      jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER site_settings_set_updated_at
    BEFORE UPDATE ON site_settings
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
