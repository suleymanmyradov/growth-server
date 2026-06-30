-- Email verification + OAuth account linking.
-- password_hash becomes nullable so OAuth-only users (Google/Apple) can exist
-- without a local password. Email verification tokens reuse the existing Redis
-- pattern (see repository/password_reset.go), so no auth_tokens table is needed.

ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;

ALTER TABLE users ADD COLUMN email_verified boolean NOT NULL DEFAULT false;

-- One user may link multiple OAuth providers. provider_uid is the provider's
-- stable subject identifier ("sub" for Google/Apple).
CREATE TABLE user_oauth_accounts (
    id            uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider      varchar(20)  NOT NULL,   -- 'google' | 'apple'
    provider_uid  varchar(255) NOT NULL,
    email         varchar(255),
    created_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider, provider_uid)
);

CREATE INDEX user_oauth_accounts_user_id_idx ON user_oauth_accounts (user_id);
