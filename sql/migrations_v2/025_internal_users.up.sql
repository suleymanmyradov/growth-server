-- Internal admin/staff users for the admin panel.
-- Separate from the public users table to keep admin auth isolated.
CREATE TABLE internal_users (
    id            uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    email         varchar(255) NOT NULL UNIQUE,
    password_hash varchar(255) NOT NULL,
    full_name     varchar(100) NOT NULL,
    role          varchar(50)  NOT NULL DEFAULT 'admin',
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER internal_users_set_updated_at
    BEFORE UPDATE ON internal_users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Seed a default admin user (password: admin123)
-- The hash is generated with bcrypt cost 10.
INSERT INTO internal_users (email, password_hash, full_name, role)
VALUES ('admin@growth.app', '$2b$10$9W.x8XhNL7YdBnnEURKtXe6Dhc.w.TgcIWPGJ/m1HFUI6oCvrvUji', 'Admin', 'admin')
ON CONFLICT (email) DO NOTHING;
