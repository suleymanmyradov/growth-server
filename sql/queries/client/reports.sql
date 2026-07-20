-- name: CreateReport :one
INSERT INTO reports (reporter_id, target_id, target_type, category, title, description, email, attachments)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, reporter_id, target_id, target_type, category, title, description, email,
          status, admin_notes, close_reason, attachments, created_at, updated_at;

-- name: GetReportByID :one
SELECT id, reporter_id, target_id, target_type, category, title, description, email,
       status, admin_notes, close_reason, attachments, created_at, updated_at
FROM reports
WHERE id = $1;

-- name: GetReportStatus :one
SELECT status, updated_at
FROM reports
WHERE id = $1;

-- name: ListReports :many
SELECT id, reporter_id, target_id, target_type, category, title, description, email,
       status, admin_notes, close_reason, attachments, created_at, updated_at
FROM reports
WHERE (sqlc.arg(status)::text = '' OR status = sqlc.arg(status)::text)
  AND (sqlc.arg(category)::text = '' OR category = sqlc.arg(category)::text)
  AND (sqlc.arg(reporter_id)::uuid = '00000000-0000-0000-0000-000000000000' OR reporter_id = sqlc.arg(reporter_id)::uuid)
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountReports :one
SELECT COUNT(*)
FROM reports
WHERE (sqlc.arg(status)::text = '' OR status = sqlc.arg(status)::text)
  AND (sqlc.arg(category)::text = '' OR category = sqlc.arg(category)::text)
  AND (sqlc.arg(reporter_id)::uuid = '00000000-0000-0000-0000-000000000000' OR reporter_id = sqlc.arg(reporter_id)::uuid);

-- name: UpdateReportStatus :one
UPDATE reports
SET status = $2,
    admin_notes = $3,
    updated_at = now()
WHERE id = $1
RETURNING id, reporter_id, target_id, target_type, category, title, description, email,
          status, admin_notes, close_reason, attachments, created_at, updated_at;

-- name: CloseReport :one
UPDATE reports
SET status = 'closed',
    close_reason = $2,
    admin_notes = $3,
    updated_at = now()
WHERE id = $1
RETURNING id, reporter_id, target_id, target_type, category, title, description, email,
          status, admin_notes, close_reason, attachments, created_at, updated_at;

-- name: CreateReportComment :one
INSERT INTO report_comments (report_id, user_id, comment, is_admin)
VALUES ($1, $2, $3, $4)
RETURNING id, report_id, user_id, comment, is_admin, created_at;

-- name: ListReportComments :many
SELECT id, report_id, user_id, comment, is_admin, created_at
FROM report_comments
WHERE report_id = $1
ORDER BY created_at ASC;
