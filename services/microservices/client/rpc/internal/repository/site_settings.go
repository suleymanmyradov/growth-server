package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/trace"
)

// SiteSettingsRepo implements ISiteSettings interface
type SiteSettingsRepo struct {
	db *db.Queries
}

// NewSiteSettingsRepo creates a new SiteSettingsRepo instance
func NewSiteSettingsRepo(db *db.Queries) *SiteSettingsRepo {
	return &SiteSettingsRepo{db: db}
}

// WithTx returns a new SiteSettingsRepo backed by the given transaction.
func (r *SiteSettingsRepo) WithTx(tx pgx.Tx) *SiteSettingsRepo {
	return &SiteSettingsRepo{db: r.db.WithTx(tx)}
}

func (r *SiteSettingsRepo) GetSiteSetting(ctx context.Context, key string) (db.SiteSetting, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "SiteSettingsRepo.GetSiteSetting")
	defer span.End()

	return r.db.GetSiteSetting(ctx, key)
}

func (r *SiteSettingsRepo) ListSiteSettings(ctx context.Context, keys []string) ([]db.SiteSetting, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "SiteSettingsRepo.ListSiteSettings")
	defer span.End()

	return r.db.ListSiteSettings(ctx, keys)
}

func (r *SiteSettingsRepo) ListAllSiteSettings(ctx context.Context) ([]db.SiteSetting, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "SiteSettingsRepo.ListAllSiteSettings")
	defer span.End()

	return r.db.ListAllSiteSettings(ctx)
}

func (r *SiteSettingsRepo) UpsertSiteSetting(ctx context.Context, key string, value []byte) (db.SiteSetting, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "SiteSettingsRepo.UpsertSiteSetting")
	defer span.End()

	return r.db.UpsertSiteSetting(ctx, key, value)
}

func (r *SiteSettingsRepo) DeleteSiteSetting(ctx context.Context, key string) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "SiteSettingsRepo.DeleteSiteSetting")
	defer span.End()

	return r.db.DeleteSiteSetting(ctx, key)
}
