package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
	"go.opentelemetry.io/otel"
)

// PushTicketsRepo owns push_tickets queries. Owned exclusively by the
// notifications service (see scripts/check-table-ownership.sh).
type PushTicketsRepo struct {
	db *db.Queries
}

func NewPushTicketsRepo(db *db.Queries) *PushTicketsRepo {
	return &PushTicketsRepo{db: db}
}

// WithTx returns a new PushTicketsRepo backed by the given transaction.
func (r *PushTicketsRepo) WithTx(tx pgx.Tx) *PushTicketsRepo {
	return &PushTicketsRepo{db: r.db.WithTx(tx)}
}

// CreatePushTicket persists a ticket for async receipt processing. Idempotent
// via ON CONFLICT DO NOTHING on (ticket_id, push_token).
func (r *PushTicketsRepo) CreatePushTicket(ctx context.Context, ticketID, pushToken string, userID, notificationID uuid.UUID) error {
	ctx, span := otel.Tracer("notifications").Start(ctx, "PushTicketsRepo.CreatePushTicket")
	defer span.End()
	return r.db.CreatePushTicket(ctx, ticketID, pushToken, userID, notificationID)
}

// ListPendingPushTickets returns up to limit tickets that have not yet had
// their receipts checked.
func (r *PushTicketsRepo) ListPendingPushTickets(ctx context.Context, limit int32) ([]db.PushTicket, error) {
	ctx, span := otel.Tracer("notifications").Start(ctx, "PushTicketsRepo.ListPendingPushTickets")
	defer span.End()
	return r.db.ListPendingPushTickets(ctx, limit)
}

// MarkPushTicketReceiptOK marks a ticket's receipt as successfully delivered.
func (r *PushTicketsRepo) MarkPushTicketReceiptOK(ctx context.Context, ticketID, pushToken string) error {
	ctx, span := otel.Tracer("notifications").Start(ctx, "PushTicketsRepo.MarkPushTicketReceiptOK")
	defer span.End()
	return r.db.MarkPushTicketReceiptOK(ctx, ticketID, pushToken)
}

// MarkPushTicketReceiptError marks a ticket's receipt as errored and records
// the error detail (e.g. "DeviceNotRegistered").
func (r *PushTicketsRepo) MarkPushTicketReceiptError(ctx context.Context, ticketID, pushToken, receiptError string) error {
	ctx, span := otel.Tracer("notifications").Start(ctx, "PushTicketsRepo.MarkPushTicketReceiptError")
	defer span.End()
	var errPtr *string
	if receiptError != "" {
		errPtr = &receiptError
	}
	return r.db.MarkPushTicketReceiptError(ctx, ticketID, pushToken, errPtr)
}

// DeleteOldPushTickets removes processed tickets older than the given threshold.
func (r *PushTicketsRepo) DeleteOldPushTickets(ctx context.Context, before pgtype.Timestamptz) error {
	ctx, span := otel.Tracer("notifications").Start(ctx, "PushTicketsRepo.DeleteOldPushTickets")
	defer span.End()
	return r.db.DeleteOldPushTickets(ctx, before)
}
