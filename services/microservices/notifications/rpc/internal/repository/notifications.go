package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
	"go.opentelemetry.io/otel"
)

type NotificationsRepo struct {
	db *db.Queries
}

func NewNotificationsRepo(db *db.Queries) *NotificationsRepo {
	return &NotificationsRepo{db: db}
}

func (r *NotificationsRepo) ListUnreadNotifications(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.ListUnreadNotificationsRow, error) {
	ctx, span := otel.Tracer("notifications").Start(ctx, "NotificationsRepo.ListUnreadNotifications")
	defer span.End()
	return r.db.ListUnreadNotifications(ctx, db.ListUnreadNotificationsParams{UserID: userID, Limit: limit, Offset: offset})
}

func (r *NotificationsRepo) ListNotificationsForUser(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.ListNotificationsForUserRow, error) {
	ctx, span := otel.Tracer("notifications").Start(ctx, "NotificationsRepo.ListNotificationsForUser")
	defer span.End()
	return r.db.ListNotificationsForUser(ctx, db.ListNotificationsForUserParams{UserID: userID, Limit: limit, Offset: offset})
}

func (r *NotificationsRepo) GetNotificationByID(ctx context.Context, id uuid.UUID) (db.GetNotificationRow, error) {
	ctx, span := otel.Tracer("notifications").Start(ctx, "NotificationsRepo.GetNotificationByID")
	defer span.End()
	return r.db.GetNotification(ctx, id)
}

func (r *NotificationsRepo) CreateNotification(ctx context.Context, params db.CreateNotificationParams) (db.CreateNotificationRow, error) {
	ctx, span := otel.Tracer("notifications").Start(ctx, "NotificationsRepo.CreateNotification")
	defer span.End()
	return r.db.CreateNotification(ctx, params)
}

func (r *NotificationsRepo) MarkNotificationRead(ctx context.Context, id uuid.UUID) (db.MarkNotificationReadRow, error) {
	ctx, span := otel.Tracer("notifications").Start(ctx, "NotificationsRepo.MarkNotificationRead")
	defer span.End()
	return r.db.MarkNotificationRead(ctx, id)
}

func (r *NotificationsRepo) MarkAllNotificationsRead(ctx context.Context, userID uuid.UUID) error {
	ctx, span := otel.Tracer("notifications").Start(ctx, "NotificationsRepo.MarkAllNotificationsRead")
	defer span.End()
	return r.db.MarkAllNotificationsRead(ctx, userID)
}

func (r *NotificationsRepo) DeleteNotification(ctx context.Context, id uuid.UUID) error {
	ctx, span := otel.Tracer("notifications").Start(ctx, "NotificationsRepo.DeleteNotification")
	defer span.End()
	return r.db.DeleteNotification(ctx, id)
}

func (r *NotificationsRepo) CountNotificationsByUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	ctx, span := otel.Tracer("notifications").Start(ctx, "NotificationsRepo.CountNotificationsByUser")
	defer span.End()
	return r.db.CountNotificationsByUser(ctx, userID)
}

func (r *NotificationsRepo) GetUnreadCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	ctx, span := otel.Tracer("notifications").Start(ctx, "NotificationsRepo.GetUnreadCount")
	defer span.End()
	return r.db.GetUnreadCount(ctx, userID)
}
