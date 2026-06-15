CREATE TABLE habits (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category_id uuid REFERENCES categories(id) ON DELETE SET NULL,
    name        varchar(100) NOT NULL CHECK (length(trim(name)) > 0),
    description text CHECK (length(description) <= 5000),
    streak      integer NOT NULL DEFAULT 0 CHECK (streak >= 0),
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_habits_user ON habits (user_id, created_at DESC);

CREATE TRIGGER habits_set_updated_at
    BEFORE UPDATE ON habits
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
