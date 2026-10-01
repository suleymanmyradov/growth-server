package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/repository/db"
	"github.com/zeromicro/go-zero/core/trace"
)

// MfaRepo wraps the adminway sqlc queries for TOTP secrets, pre-auth
// tickets, and backup codes.
type MfaRepo struct {
	db *db.Queries
}

func NewMfaRepo(dbq *db.Queries) *MfaRepo {
	return &MfaRepo{db: dbq}
}

func (r *MfaRepo) SetTotpSecret(ctx context.Context, userID uuid.UUID, encryptedSecret string) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "MfaRepo.SetTotpSecret")
	defer span.End()

	return r.db.SetInternalUserTotpSecret(ctx, userID, &encryptedSecret)
}

func (r *MfaRepo) EnableTotp(ctx context.Context, userID uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "MfaRepo.EnableTotp")
	defer span.End()

	return r.db.EnableInternalUserTotp(ctx, userID)
}

func (r *MfaRepo) DisableTotp(ctx context.Context, userID uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "MfaRepo.DisableTotp")
	defer span.End()

	return r.db.DisableInternalUserTotp(ctx, userID)
}

func (r *MfaRepo) CreateTicket(ctx context.Context, userID uuid.UUID, tokenHash, purpose string, expiresAt time.Time) (db.AdminMfaTicket, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "MfaRepo.CreateTicket")
	defer span.End()

	return r.db.CreateAdminMfaTicket(ctx, userID, tokenHash, purpose, pgtype.Timestamptz{Time: expiresAt, Valid: true})
}

func (r *MfaRepo) GetTicketByHash(ctx context.Context, tokenHash string) (db.AdminMfaTicket, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "MfaRepo.GetTicketByHash")
	defer span.End()

	return r.db.GetAdminMfaTicketByHash(ctx, tokenHash)
}

func (r *MfaRepo) IncrementTicketAttempts(ctx context.Context, ticketID uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "MfaRepo.IncrementTicketAttempts")
	defer span.End()

	return r.db.IncrementAdminMfaTicketAttempts(ctx, ticketID)
}

func (r *MfaRepo) DeleteTicket(ctx context.Context, ticketID uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "MfaRepo.DeleteTicket")
	defer span.End()

	return r.db.DeleteAdminMfaTicket(ctx, ticketID)
}

func (r *MfaRepo) DeleteTicketsForUser(ctx context.Context, userID uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "MfaRepo.DeleteTicketsForUser")
	defer span.End()

	return r.db.DeleteAdminMfaTicketsForUser(ctx, userID)
}

// ConsumeBackupCode marks the code used and reports whether an unused code
// matched. A no-row result is a normal "wrong code" outcome, not an error.
func (r *MfaRepo) ConsumeBackupCode(ctx context.Context, userID uuid.UUID, codeHash string) (bool, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "MfaRepo.ConsumeBackupCode")
	defer span.End()

	_, err := r.db.ConsumeAdminMfaBackupCode(ctx, userID, codeHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (r *MfaRepo) InsertBackupCode(ctx context.Context, userID uuid.UUID, codeHash string) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "MfaRepo.InsertBackupCode")
	defer span.End()

	return r.db.InsertAdminMfaBackupCode(ctx, userID, codeHash)
}

func (r *MfaRepo) DeleteBackupCodesForUser(ctx context.Context, userID uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "MfaRepo.DeleteBackupCodesForUser")
	defer span.End()

	return r.db.DeleteAdminMfaBackupCodesForUser(ctx, userID)
}
