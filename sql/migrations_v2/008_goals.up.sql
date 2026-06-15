CREATE TABLE goals (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category_id uuid REFERENCES categories(id) ON DELETE SET NULL,
    title       varchar(200) NOT NULL CHECK (length(trim(title)) > 0),
    description text,
    status      text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'completed', 'archived')),
    progress    integer NOT NULL DEFAULT 0 CHECK (progress BETWEEN 0 AND 100),
    due_date    timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

-- serves "list my goals", "list my active goals", both newest-first
CREATE INDEX idx_goals_user ON goals (user_id, status, created_at DESC);

CREATE TRIGGER goals_set_updated_at
    BEFORE UPDATE ON goals
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
