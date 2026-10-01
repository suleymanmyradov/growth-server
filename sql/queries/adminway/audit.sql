-- name: InsertAdminAuditLog :exec
INSERT INTO admin_audit_log (admin_id, admin_name, method, path, status_code, ip, user_agent, latency_ms)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);
