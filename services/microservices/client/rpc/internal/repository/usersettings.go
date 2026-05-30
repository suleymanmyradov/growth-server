package repository

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/trace"
)

// userSettingsCacheEntry holds a cached settings row with its TTL.
type userSettingsCacheEntry struct {
	settings db.GetUserSettingsRow
	expires  time.Time
}

// UserSettingsRepo implements IUserSettings interface.
// It includes an in-memory cache for GetUserSettings to avoid repeated DB lookups.
type UserSettingsRepo struct {
	db    *db.Queries
	cache map[string]userSettingsCacheEntry
	mu    sync.RWMutex
	ttl   time.Duration
}

// NewUserSettingsRepo creates a new UserSettingsRepo instance.
func NewUserSettingsRepo(dbq *db.Queries) *UserSettingsRepo {
	return &UserSettingsRepo{
		db:    dbq,
		cache: make(map[string]userSettingsCacheEntry),
		ttl:   5 * time.Minute,
	}
}

// WithTx returns a new UserSettingsRepo backed by the given transaction.
func (r *UserSettingsRepo) WithTx(tx pgx.Tx) *UserSettingsRepo {
	return &UserSettingsRepo{
		db:    r.db.WithTx(tx),
		cache: r.cache,
		ttl:   r.ttl,
	}
}

func (r *UserSettingsRepo) GetUserSettings(ctx context.Context, userID uuid.UUID) (db.GetUserSettingsRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "UserSettingsRepo.GetUserSettings")
	defer span.End()

	key := userID.String()

	r.mu.RLock()
	entry, ok := r.cache[key]
	r.mu.RUnlock()

	if ok && time.Now().Before(entry.expires) {
		return entry.settings, nil
	}

	settings, err := r.db.GetUserSettings(ctx, userID)
	if err != nil {
		return db.GetUserSettingsRow{}, err
	}

	r.mu.Lock()
	r.cache[key] = userSettingsCacheEntry{settings: settings, expires: time.Now().Add(r.ttl)}
	r.mu.Unlock()

	return settings, nil
}

// InvalidateCache removes the cached entry for the given user.
// Call this after any update to ensure subsequent reads see fresh data.
func (r *UserSettingsRepo) InvalidateCache(userID uuid.UUID) {
	r.mu.Lock()
	delete(r.cache, userID.String())
	r.mu.Unlock()
}

func (r *UserSettingsRepo) GetUserSettingsByID(ctx context.Context, id uuid.UUID) (db.GetUserSettingsByIDRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "UserSettingsRepo.GetUserSettingsByID")
	defer span.End()

	return r.db.GetUserSettingsByID(ctx, id)
}

func (r *UserSettingsRepo) CreateUserSettings(ctx context.Context, params db.CreateUserSettingsParams) (db.CreateUserSettingsRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "UserSettingsRepo.CreateUserSettings")
	defer span.End()

	return r.db.CreateUserSettings(ctx, params)
}

func (r *UserSettingsRepo) UpdateUserSettings(ctx context.Context, params db.UpdateUserSettingsParams) (db.UpdateUserSettingsRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "UserSettingsRepo.UpdateUserSettings")
	defer span.End()

	result, err := r.db.UpdateUserSettings(ctx, params)
	if err == nil {
		r.InvalidateCache(params.UserID)
	}
	return result, err
}

func (r *UserSettingsRepo) DeleteUserSettings(ctx context.Context, userID uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "UserSettingsRepo.DeleteUserSettings")
	defer span.End()

	err := r.db.DeleteUserSettings(ctx, userID)
	if err == nil {
		r.InvalidateCache(userID)
	}
	return err
}

func (r *UserSettingsRepo) UpdateOnboardingSettings(ctx context.Context, params db.UpdateOnboardingSettingsParams) (db.UpdateOnboardingSettingsRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "UserSettingsRepo.UpdateOnboardingSettings")
	defer span.End()

	result, err := r.db.UpdateOnboardingSettings(ctx, params)
	if err == nil {
		r.InvalidateCache(params.UserID)
	}
	return result, err
}


