-- Ensure pgcrypto and uuid_generate_v7() exist (safety if migration 000 not applied)
CREATE EXTENSION IF NOT EXISTS pgcrypto;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_proc WHERE proname = 'uuid_generate_v7' AND pg_function_is_visible(oid)
    ) THEN
        CREATE OR REPLACE FUNCTION uuid_generate_v7()
        RETURNS uuid AS $fn$
        DECLARE
            unix_ts_ms BIGINT;
            rand_bytes BYTEA;
            uuid_bytes BYTEA;
        BEGIN
            unix_ts_ms := FLOOR(EXTRACT(EPOCH FROM clock_timestamp()) * 1000);
            rand_bytes := gen_random_bytes(10);
            uuid_bytes := decode(lpad(to_hex(unix_ts_ms), 12, '0'), 'hex') || rand_bytes;
            uuid_bytes := set_byte(uuid_bytes, 6, (get_byte(uuid_bytes, 6) & 0x0f) | 0x70);
            uuid_bytes := set_byte(uuid_bytes, 8, (get_byte(uuid_bytes, 8) & 0x3f) | 0x80);
            RETURN encode(uuid_bytes, 'hex')::uuid;
        END;
        $fn$ LANGUAGE plpgsql VOLATILE;
    END IF;
END$$;

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    full_name VARCHAR(100) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Trigger to update updated_at timestamp
CREATE TRIGGER update_users_updated_at 
    BEFORE UPDATE ON users 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();
