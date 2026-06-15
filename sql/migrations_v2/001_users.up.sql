-- Identity + public profile in one table (old `profiles` table merged in).
CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    username      varchar(50)  NOT NULL UNIQUE,
    email         varchar(255) NOT NULL UNIQUE,
    password_hash varchar(255) NOT NULL,
    full_name     varchar(100) NOT NULL,
    bio           text,
    location      varchar(100),
    website       varchar(255),
    interests     text[],
    avatar_url    varchar(500),
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),

    -- lowercase, must start with a letter
    CONSTRAINT users_username_format CHECK (username ~ '^[a-z][a-z0-9_-]*$')
);

CREATE TRIGGER users_set_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
