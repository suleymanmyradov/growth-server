-- Push ticket persistence for async Expo receipt processing.
-- Owned exclusively by the notifications service.

-- name: CreatePushTicket :exec
INSERT INTO push_tickets (ticket_id, push_token, user_id, notification_id)
VALUES ($1, $2, $3, $4)
ON CONFLICT (ticket_id, push_token) DO NOTHING;

-- name: ListPendingPushTickets :many
SELECT id, ticket_id, push_token, user_id, notification_id, receipt_status, receipt_error, receipt_checked_at, created_at
FROM push_tickets
WHERE receipt_status = 'pending'
ORDER BY created_at ASC
LIMIT $1;

-- name: MarkPushTicketReceiptOK :exec
UPDATE push_tickets
SET receipt_status = 'ok', receipt_checked_at = now()
WHERE ticket_id = $1 AND push_token = $2;

-- name: MarkPushTicketReceiptError :exec
UPDATE push_tickets
SET receipt_status = 'error', receipt_error = $3, receipt_checked_at = now()
WHERE ticket_id = $1 AND push_token = $2;

-- name: DeleteOldPushTickets :exec
DELETE FROM push_tickets
WHERE created_at < $1 AND receipt_status != 'pending';
