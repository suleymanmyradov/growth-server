package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/trace"
)

// NotificationsRepo implements INotifications interface
type NotificationsRepo struct {
	db *db.Queries
}

// NewNotificationsRepo creates a new NotificationsRepo instance
func NewNotificationsRepo(db *db.Queries) *NotificationsRepo {
	return &NotificationsRepo{db: db}
}

func (r *NotificationsRepo) ListNotifications(ctx context.Context, limit, offset int32) ([]db.Notification, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "NotificationsRepo.ListNotifications")
	defer span.End()

	return r.db.ListNotifications(ctx, db.ListNotificationsParams{
		Limit:  limit,
		Offset: offset,
	})
}

func (r *NotificationsRepo) ListNotificationsByUser(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.Notification, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "NotificationsRepo.ListNotificationsByUser")
	defer span.End()

	return r.db.ListNotificationsByUser(ctx, db.ListNotificationsByUserParams{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	})
}

func (r *NotificationsRepo) ListUnreadNotifications(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.Notification, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "NotificationsRepo.ListUnreadNotifications")
	defer span.End()

	return r.db.ListUnreadNotifications(ctx, db.ListUnreadNotificationsParams{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	})
}

func (r *NotificationsRepo) ListNotificationsForUser(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.Notification, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "NotificationsRepo.ListNotificationsForUser")
	defer span.End()

	return r.db.ListNotificationsForUser(ctx, db.ListNotificationsForUserParams{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	})
}

func (r *NotificationsRepo) ListNotificationsByType(ctx context.Context, userID uuid.UUID, itemType string, limit, offset int32) ([]db.Notification, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "NotificationsRepo.ListNotificationsByType")
	defer span.End()

	return r.db.ListNotificationsByType(ctx, db.ListNotificationsByTypeParams{
		UserID:   userID,
		ItemType: itemType,
		Limit:    limit,
		Offset:   offset,
	})
}

func (r *NotificationsRepo) GetNotification(ctx context.Context, id uuid.UUID) (db.Notification, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "NotificationsRepo.GetNotification")
	defer span.End()

	return r.db.GetNotification(ctx, id)
}

func (r *NotificationsRepo) CreateNotification(ctx context.Context, params db.CreateNotificationParams) (db.Notification, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "NotificationsRepo.CreateNotification")
	defer span.End()

	return r.db.CreateNotification(ctx, params)
}

func (r *NotificationsRepo) MarkNotificationRead(ctx context.Context, id uuid.UUID) (db.Notification, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "NotificationsRepo.MarkNotificationRead")
	defer span.End()

	return r.db.MarkNotificationRead(ctx, id)
}

func (r *NotificationsRepo) MarkAllNotificationsRead(ctx context.Context, userID uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "NotificationsRepo.MarkAllNotificationsRead")
	defer span.End()

	return r.db.MarkAllNotificationsRead(ctx, userID)
}

func (r *NotificationsRepo) DeleteNotification(ctx context.Context, id uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "NotificationsRepo.DeleteNotification")
	defer span.End()

	return r.db.DeleteNotification(ctx, id)
}

func (r *NotificationsRepo) DeleteAllNotificationsByUser(ctx context.Context, userID uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "NotificationsRepo.DeleteAllNotificationsByUser")
	defer span.End()

	return r.db.DeleteAllNotificationsByUser(ctx, userID)
}

func (r *NotificationsRepo) CountNotifications(ctx context.Context) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "NotificationsRepo.CountNotifications")
	defer span.End()

	return r.db.CountNotifications(ctx)
}

func (r *NotificationsRepo) CountNotificationsByUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "NotificationsRepo.CountNotificationsByUser")
	defer span.End()

	return r.db.CountNotificationsByUser(ctx, userID)
}

func (r *NotificationsRepo) CountUnreadNotifications(ctx context.Context, userID uuid.UUID) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "NotificationsRepo.CountUnreadNotifications")
	defer span.End()

	return r.db.CountUnreadNotifications(ctx, userID)
}

func (r *NotificationsRepo) GetUnreadCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "NotificationsRepo.GetUnreadCount")
	defer span.End()

	return r.db.GetUnreadCount(ctx, userID)
}
