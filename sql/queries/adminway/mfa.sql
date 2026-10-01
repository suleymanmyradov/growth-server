-- Admin MFA: pending TOTP secrets on internal_users, pre-auth tickets, and
-- one-time backup codes. Table rationale lives in migration 068.

-- Pending/active TOTP secret lifecycle. The secret written by
-- SetInternalUserTotpSecret is inert until EnableInternalUserTotp confirms it
-- with a valid code — login only requires MFA once totp_enabled_at is set.
-- name: SetInternalUserTotpSecret :exec
UPDATE internal_users
SET totp_secret_encrypted = $2
WHERE id = $1;

-- name: EnableInternalUserTotp :exec
UPDATE internal_users
SET totp_enabled_at = now()
WHERE id = $1;

-- name: DisableInternalUserTotp :exec
UPDATE internal_users
SET totp_secret_encrypted = NULL,
    totp_enabled_at = NULL
WHERE id = $1;

-- Tickets bridge the password step and the TOTP step of login. Only a SHA-256
-- hash of the client-facing token is stored.
-- name: CreateAdminMfaTicket :one
INSERT INTO admin_mfa_tickets (user_id, token_hash, purpose, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING id, user_id, token_hash, purpose, attempts, expires_at, created_at;

-- name: GetAdminMfaTicketByHash :one
SELECT id, user_id, token_hash, purpose, attempts, expires_at, created_at
FROM admin_mfa_tickets
WHERE token_hash = $1;

-- name: IncrementAdminMfaTicketAttempts :exec
UPDATE admin_mfa_tickets
SET attempts = attempts + 1
WHERE id = $1;

-- name: DeleteAdminMfaTicket :exec
DELETE FROM admin_mfa_tickets WHERE id = $1;

-- name: DeleteAdminMfaTicketsForUser :exec
DELETE FROM admin_mfa_tickets WHERE user_id = $1;

-- name: DeleteExpiredAdminMfaTickets :exec
DELETE FROM admin_mfa_tickets WHERE expires_at < now();

-- Backup codes are stored as SHA-256 hashes. Consume returns a row only when
-- an unused code matches, which is also the validity signal.
-- name: InsertAdminMfaBackupCode :exec
INSERT INTO admin_mfa_backup_codes (user_id, code_hash)
VALUES ($1, $2);

-- name: ConsumeAdminMfaBackupCode :one
UPDATE admin_mfa_backup_codes
SET used_at = now()
WHERE user_id = $1 AND code_hash = $2 AND used_at IS NULL
RETURNING id;

-- name: DeleteAdminMfaBackupCodesForUser :exec
DELETE FROM admin_mfa_backup_codes WHERE user_id = $1;
