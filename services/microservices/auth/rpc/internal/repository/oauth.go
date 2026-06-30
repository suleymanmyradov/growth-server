package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/trace"
)

// OauthRepo implements IOauth.
type OauthRepo struct {
	db *db.Queries
}

func NewOauthRepo(dbq *db.Queries) *OauthRepo {
	return &OauthRepo{db: dbq}
}

// WithTx returns a new OauthRepo backed by the given transaction.
func (r *OauthRepo) WithTx(tx pgx.Tx) *OauthRepo {
	return &OauthRepo{db: r.db.WithTx(tx)}
}

func (r *OauthRepo) GetOAuthAccount(ctx context.Context, provider, providerUID string) (db.UserOauthAccount, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "OauthRepo.GetOAuthAccount")
	defer span.End()

	return r.db.GetOAuthAccount(ctx, provider, providerUID)
}

func (r *OauthRepo) CreateOAuthAccount(ctx context.Context, userID uuid.UUID, provider, providerUID string, email *string) (db.UserOauthAccount, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "OauthRepo.CreateOAuthAccount")
	defer span.End()

	return r.db.CreateOAuthAccount(ctx, userID, provider, providerUID, email)
}
