-- Reports & report comments — user-submitted bug reports, feedback, and abuse
-- reports, plus an admin-facing comment thread per report. Owned by the client
-- service. Statuses: open, in_review, resolved, closed. Categories: bug,
-- feedback, abuse (free-form string, validated at the logic layer).

CREATE TABLE reports (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    reporter_id uuid NOT NULL,
    target_id   varchar(100),
    target_type varchar(50) NOT NULL DEFAULT 'general',
    category    varchar(50) NOT NULL,
    title       varchar(200) NOT NULL CHECK (length(trim(title)) > 0),
    description text NOT NULL CHECK (length(trim(description)) > 0),
    email       varchar(255),
    status      varchar(20) NOT NULL DEFAULT 'open',
    admin_notes text,
    close_reason text,
    attachments text[] NOT NULL DEFAULT '{}',
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_reports_status ON reports (status);
CREATE INDEX idx_reports_category ON reports (category);
CREATE INDEX idx_reports_reporter ON reports (reporter_id);
CREATE INDEX idx_reports_created_at ON reports (created_at DESC);

CREATE TRIGGER reports_set_updated_at
    BEFORE UPDATE ON reports
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE report_comments (
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    report_id  uuid NOT NULL REFERENCES reports(id) ON DELETE CASCADE,
    user_id    uuid NOT NULL,
    comment    text NOT NULL CHECK (length(trim(comment)) > 0),
    is_admin   boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_report_comments_report ON report_comments (report_id, created_at);
