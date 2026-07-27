package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
	"go.opentelemetry.io/otel"
)

// DevicesRepo owns notification_devices queries. Owned exclusively by the
// notifications service (see scripts/check-table-ownership.sh).
type DevicesRepo struct {
	db *db.Queries
}

func NewDevicesRepo(db *db.Queries) *DevicesRepo {
	return &DevicesRepo{db: db}
}

// WithTx returns a new DevicesRepo backed by the given transaction.
func (r *DevicesRepo) WithTx(tx pgx.Tx) *DevicesRepo {
	return &DevicesRepo{db: r.db.WithTx(tx)}
}

// UpsertDevice inserts a new device registration or updates the push token +
// metadata for an existing (installation_id, user_id) pair. Token rotation is
// handled by the ON CONFLICT UPDATE branch. enabled is reset to true.
func (r *DevicesRepo) UpsertDevice(ctx context.Context, arg db.UpsertDeviceParams) (db.NotificationDevice, error) {
	ctx, span := otel.Tracer("notifications").Start(ctx, "DevicesRepo.UpsertDevice")
	defer span.End()
	return r.db.UpsertDevice(ctx, arg)
}

// DeleteDevice removes a device registration by installation_id + user_id.
func (r *DevicesRepo) DeleteDevice(ctx context.Context, installationID string, userID uuid.UUID) error {
	ctx, span := otel.Tracer("notifications").Start(ctx, "DevicesRepo.DeleteDevice")
	defer span.End()
	return r.db.DeleteDevice(ctx, installationID, userID)
}

// ListActiveDevicesByUser returns all enabled devices for a user, used to fan
// out push notifications.
func (r *DevicesRepo) ListActiveDevicesByUser(ctx context.Context, userID uuid.UUID) ([]db.NotificationDevice, error) {
	ctx, span := otel.Tracer("notifications").Start(ctx, "DevicesRepo.ListActiveDevicesByUser")
	defer span.End()
	return r.db.ListActiveDevicesByUser(ctx, userID)
}

// DisableDeviceByToken marks a device disabled when the push provider reports
// the token as invalid (e.g. Expo receipt API DeviceNotRegistered error).
func (r *DevicesRepo) DisableDeviceByToken(ctx context.Context, pushToken string) error {
	ctx, span := otel.Tracer("notifications").Start(ctx, "DevicesRepo.DisableDeviceByToken")
	defer span.End()
	return r.db.DisableDeviceByToken(ctx, pushToken)
}

// UpdateDeviceLastSeen bumps last_seen_at for a registration/heartbeat.
func (r *DevicesRepo) UpdateDeviceLastSeen(ctx context.Context, installationID string, userID uuid.UUID) error {
	ctx, span := otel.Tracer("notifications").Start(ctx, "DevicesRepo.UpdateDeviceLastSeen")
	defer span.End()
	return r.db.UpdateDeviceLastSeen(ctx, installationID, userID)
}

// DeleteByUser removes all device registrations for a user. Called from the
// user_deleted event consumer to clean up notifications-owned device rows
// without a cross-service foreign key.
func (r *DevicesRepo) DeleteByUser(ctx context.Context, userID uuid.UUID) error {
	ctx, span := otel.Tracer("notifications").Start(ctx, "DevicesRepo.DeleteByUser")
	defer span.End()
	return r.db.DeleteDevicesByUser(ctx, userID)
}
