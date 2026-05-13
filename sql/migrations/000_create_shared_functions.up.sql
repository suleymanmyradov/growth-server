-- Shared database functions used across multiple tables
-- This migration should run first to ensure functions exist before triggers are created

-- Extension for UUID generation (uuid_generate_v7)
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Compatibility function for uuid_generate_v7() on PG versions where it is not built-in (PG <16)
CREATE OR REPLACE FUNCTION uuid_generate_v7()
RETURNS uuid AS $$
DECLARE
    unix_ts_ms BIGINT;
    rand_bytes BYTEA;
    uuid_bytes BYTEA;
BEGIN
    unix_ts_ms := FLOOR(EXTRACT(EPOCH FROM clock_timestamp()) * 1000);
    rand_bytes := gen_random_bytes(10); -- remaining 80 bits
    -- 48-bit timestamp (6 bytes) + 10 random bytes = 16 bytes
    uuid_bytes := decode(lpad(to_hex(unix_ts_ms), 12, '0'), 'hex') || rand_bytes;

    -- Set version (7)
    uuid_bytes := set_byte(uuid_bytes, 6, (get_byte(uuid_bytes, 6) & 0x0f) | 0x70);
    -- Set variant (RFC 4122)
    uuid_bytes := set_byte(uuid_bytes, 8, (get_byte(uuid_bytes, 8) & 0x3f) | 0x80);

    RETURN encode(uuid_bytes, 'hex')::uuid;
END;
$$ LANGUAGE plpgsql VOLATILE;

-- Shared trigger function for updating updated_at timestamps
-- Using CREATE OR REPLACE to ensure idempotency
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
