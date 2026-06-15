-- Shared extensions and helper functions. Run first.

CREATE EXTENSION IF NOT EXISTS pgcrypto; -- gen_random_bytes() for uuid_generate_v7

-- Time-ordered UUIDs (v7): random enough to be unguessable, ordered enough
-- to keep btree inserts local. Used as the default PK everywhere.
CREATE OR REPLACE FUNCTION uuid_generate_v7() RETURNS uuid
LANGUAGE plpgsql
AS $$
DECLARE
    unix_ts_ms BIGINT;
    uuid_bytes BYTEA;
BEGIN
    unix_ts_ms := FLOOR(EXTRACT(EPOCH FROM clock_timestamp()) * 1000);
    uuid_bytes := decode(lpad(to_hex(unix_ts_ms), 12, '0'), 'hex') || gen_random_bytes(10);
    uuid_bytes := set_byte(uuid_bytes, 6, (get_byte(uuid_bytes, 6) & 15) | 112); -- version 7
    uuid_bytes := set_byte(uuid_bytes, 8, (get_byte(uuid_bytes, 8) & 63) | 128); -- RFC 4122 variant
    RETURN encode(uuid_bytes, 'hex')::uuid;
END;
$$;

-- Generic updated_at maintenance, attached per table where it exists.
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$;
