-- Admin MFA (TOTP) for internal_users (adminway).
--
-- totp_secret_encrypted holds the AES-GCM-encrypted base32 TOTP secret
-- (encryption key lives in the service config, not the DB). It is written at
-- setup time but only becomes authoritative once totp_enabled_at is set by a
-- successful confirm — a pending secret alone does not require MFA at login.
--
-- admin_mfa_tickets are the pre-auth bridge between the password step and the
-- TOTP step of login (purpose 'verify'), and between login and the setup
-- endpoints when enrollment is forced (purpose 'enroll'). The client-facing
-- token is a random 256-bit value; only its SHA-256 hash is stored so a DB
-- read alone cannot replay a live ticket. Single-use, short TTL, attempts
-- counter bounds per-ticket code guessing.
--
-- admin_mfa_backup_codes stores SHA-256 hashes of one-time recovery codes;
-- used_at is set on consume so a code works exactly once.

ALTER TABLE internal_users
    ADD COLUMN totp_secret_encrypted text,
    ADD COLUMN totp_enabled_at      timestamptz;

CREATE TABLE admin_mfa_tickets (
    id         uuid        NOT NULL DEFAULT uuid_generate_v7(),
    user_id    uuid        NOT NULL REFERENCES internal_users (id) ON DELETE CASCADE,
    token_hash text        NOT NULL UNIQUE,
    purpose    text        NOT NULL CHECK (purpose IN ('verify', 'enroll')),
    attempts   int         NOT NULL DEFAULT 0,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (id)
);

CREATE TABLE admin_mfa_backup_codes (
    id         uuid        NOT NULL DEFAULT uuid_generate_v7(),
    user_id    uuid        NOT NULL REFERENCES internal_users (id) ON DELETE CASCADE,
    code_hash  text        NOT NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (id)
);

CREATE INDEX idx_admin_mfa_backup_codes_user_id ON admin_mfa_backup_codes (user_id);
