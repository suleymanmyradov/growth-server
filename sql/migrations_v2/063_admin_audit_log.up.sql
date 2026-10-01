-- Admin action audit log (adminway).
--
-- Every request that reaches adminway — authenticated admin actions and
-- unauthenticated attempts (login, refresh) — is recorded by the audit
-- middleware. Writes are buffered and inserted asynchronously so request
-- latency and audit durability never couple (see
-- adminapi/internal/middleware/auditlog.go).
--
-- admin_id/admin_name are nullable: failed logins and rate-limited requests
-- carry no verified principal. ip is text (not inet) because it may hold a
-- forwarded-for chain.

CREATE TABLE admin_audit_log (
    id          uuid        NOT NULL DEFAULT gen_random_uuid(),
    admin_id    uuid,
    admin_name  text,
    method      text        NOT NULL,
    path        text        NOT NULL,
    status_code int         NOT NULL,
    ip          text,
    user_agent  text,
    latency_ms  int         NOT NULL DEFAULT 0,
    created_at  timestamptz NOT NULL DEFAULT now(),

    PRIMARY KEY (id)
);

-- Forensic queries are "what did admin X do" and "what happened recently".
CREATE INDEX idx_admin_audit_log_admin_id_created ON admin_audit_log (admin_id, created_at DESC);
CREATE INDEX idx_admin_audit_log_created_at ON admin_audit_log (created_at DESC);
