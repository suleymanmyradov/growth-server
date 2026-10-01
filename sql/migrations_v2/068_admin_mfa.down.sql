DROP TABLE admin_mfa_backup_codes;
DROP TABLE admin_mfa_tickets;

ALTER TABLE internal_users
    DROP COLUMN totp_enabled_at,
    DROP COLUMN totp_secret_encrypted;
