-- Push ticket persistence for async Expo receipt processing.
--
-- When the notifications service sends a push via Expo, Expo returns a ticket
-- for each message. The ticket ID is later used to check the delivery receipt
-- (Expo's getReceipts API), which reports the final delivery status including
-- errors like DeviceNotRegistered that only appear in the receipt (not in the
-- initial ticket response).
--
-- This table is owned exclusively by the notifications service.

CREATE TABLE push_tickets (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ticket_id       text NOT NULL,
    push_token      text NOT NULL,
    user_id         uuid NOT NULL,
    notification_id uuid NOT NULL,
    receipt_status  text NOT NULL DEFAULT 'pending',  -- pending | ok | error
    receipt_error   text,
    receipt_checked_at timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),

    -- A ticket ID can appear for multiple tokens (rare, but Expo reuses IDs
    -- across batch sends), so the unique constraint is on (ticket_id, push_token).
    UNIQUE (ticket_id, push_token)
);

-- Index for the receipt worker: find pending tickets older than a threshold.
CREATE INDEX idx_push_tickets_receipt_status
    ON push_tickets (receipt_status, created_at)
    WHERE receipt_status = 'pending';

-- Index for cleanup of old processed tickets.
CREATE INDEX idx_push_tickets_created_at ON push_tickets (created_at);
