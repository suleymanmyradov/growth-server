DROP TABLE IF EXISTS user_oauth_accounts;
ALTER TABLE users DROP COLUMN IF EXISTS email_verified;
ALTER TABLE users ALTER COLUMN password_hash SET NOT NULL;
