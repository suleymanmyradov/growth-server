package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/repository/db"
)

type IInternalUsers interface {
	Create(ctx context.Context, email, passwordHash, fullName, role string) (db.InternalUser, error)
	GetByEmail(ctx context.Context, email string) (db.InternalUser, error)
	GetByID(ctx context.Context, id uuid.UUID) (db.InternalUser, error)
	UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) (db.InternalUser, error)
	UpdateProfile(ctx context.Context, id uuid.UUID, fullName string) (db.InternalUser, error)
}

type IMfa interface {
	SetTotpSecret(ctx context.Context, userID uuid.UUID, encryptedSecret string) error
	EnableTotp(ctx context.Context, userID uuid.UUID) error
	DisableTotp(ctx context.Context, userID uuid.UUID) error
	CreateTicket(ctx context.Context, userID uuid.UUID, tokenHash, purpose string, expiresAt time.Time) (db.AdminMfaTicket, error)
	GetTicketByHash(ctx context.Context, tokenHash string) (db.AdminMfaTicket, error)
	IncrementTicketAttempts(ctx context.Context, ticketID uuid.UUID) error
	DeleteTicket(ctx context.Context, ticketID uuid.UUID) error
	DeleteTicketsForUser(ctx context.Context, userID uuid.UUID) error
	ConsumeBackupCode(ctx context.Context, userID uuid.UUID, codeHash string) (bool, error)
	InsertBackupCode(ctx context.Context, userID uuid.UUID, codeHash string) error
	DeleteBackupCodesForUser(ctx context.Context, userID uuid.UUID) error
}

type Repository struct {
	InternalUsers IInternalUsers
	Mfa           IMfa
	Analytics     IAnalytics
}

func NewRepository(dbq *db.Queries) *Repository {
	return &Repository{
		InternalUsers: NewInternalUsersRepo(dbq),
		Mfa:           NewMfaRepo(dbq),
		Analytics:     NewAnalyticsRepo(dbq),
	}
}
