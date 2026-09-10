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

-- No seed admin: a migration-seeded password would be publicly known on every
-- fresh deployment, and the admin panel has no self-registration route
-- (removed from services/adminway/contract/main.api on purpose).
--
-- First-admin bootstrap (run once after `make migrate-up`):
--   1. Generate a bcrypt hash (htpasswd output is accepted by Go's bcrypt):
--        htpasswd -bnBC 10 "" 'YOUR_STRONG_PASSWORD' | tr -d ':\n'
--   2. Insert the admin:
--        INSERT INTO internal_users (email, password_hash, full_name, role)
--        VALUES ('admin@evolella.com', '<BCRYPT_HASH>', 'Admin', 'admin');
--   3. Log in at https://admin.evolella.com/login and delete this row's
--      shell history if it contains the password.
