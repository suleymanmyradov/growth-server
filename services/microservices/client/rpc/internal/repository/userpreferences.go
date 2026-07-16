package repository

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/trace"
)

// userPreferencesCacheEntry holds a cached preferences row with its TTL.
type userPreferencesCacheEntry struct {
	prefs   db.UserPreference
	expires time.Time
}

// UserPreferencesRepo implements IUserPreferences interface.
// It includes an in-memory cache for GetUserPreferences to avoid repeated DB lookups.
type UserPreferencesRepo struct {
	db    *db.Queries
	cache map[string]userPreferencesCacheEntry
	mu    sync.RWMutex
	ttl   time.Duration
}

// NewUserPreferencesRepo creates a new UserPreferencesRepo instance.
func NewUserPreferencesRepo(dbq *db.Queries) *UserPreferencesRepo {
	return &UserPreferencesRepo{
		db:    dbq,
		cache: make(map[string]userPreferencesCacheEntry),
		ttl:   5 * time.Minute,
	}
}

// WithTx returns a new UserPreferencesRepo backed by the given transaction.
func (r *UserPreferencesRepo) WithTx(tx pgx.Tx) *UserPreferencesRepo {
	return &UserPreferencesRepo{
		db:    r.db.WithTx(tx),
		cache: r.cache,
		ttl:   r.ttl,
	}
}

func (r *UserPreferencesRepo) GetUserPreferences(ctx context.Context, userID uuid.UUID) (db.UserPreference, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "UserPreferencesRepo.GetUserPreferences")
	defer span.End()

	key := userID.String()

	r.mu.RLock()
	entry, ok := r.cache[key]
	r.mu.RUnlock()

	if ok && time.Now().Before(entry.expires) {
		return entry.prefs, nil
	}

	prefs, err := r.db.GetUserPreferences(ctx, userID)
	if err != nil {
		return db.UserPreference{}, err
	}

	r.mu.Lock()
	r.cache[key] = userPreferencesCacheEntry{prefs: prefs, expires: time.Now().Add(r.ttl)}
	r.mu.Unlock()

	return prefs, nil
}

// InvalidateCache removes the cached entry for the given user.
// Call this after any update to ensure subsequent reads see fresh data.
func (r *UserPreferencesRepo) InvalidateCache(userID uuid.UUID) {
	r.mu.Lock()
	delete(r.cache, userID.String())
	r.mu.Unlock()
}

func (r *UserPreferencesRepo) CreateUserPreferences(ctx context.Context, theme string, language string, timezone string, userID uuid.UUID) (db.UserPreference, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "UserPreferencesRepo.CreateUserPreferences")
	defer span.End()

	return r.db.CreateUserPreferences(ctx, theme, language, timezone, userID)
}

func (r *UserPreferencesRepo) UpdateUserPreferences(ctx context.Context, userID uuid.UUID, theme string, language string, timezone string) (db.UserPreference, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "UserPreferencesRepo.UpdateUserPreferences")
	defer span.End()

	result, err := r.db.UpdateUserPreferences(ctx, userID, theme, language, timezone)
	if err == nil {
		r.InvalidateCache(userID)
	}
	return result, err
}

func (r *UserPreferencesRepo) DeleteUserPreferences(ctx context.Context, userID uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "UserPreferencesRepo.DeleteUserPreferences")
	defer span.End()

	err := r.db.DeleteUserPreferences(ctx, userID)
	if err == nil {
		r.InvalidateCache(userID)
	}
	return err
}

func (r *UserPreferencesRepo) UpdateOnboardingCompleted(ctx context.Context, userID uuid.UUID, checkInTime pgtype.Time, onboardingCompleted bool) (db.UserPreference, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "UserPreferencesRepo.UpdateOnboardingCompleted")
	defer span.End()

	result, err := r.db.UpdateOnboardingCompleted(ctx, userID, checkInTime, onboardingCompleted)
	if err == nil {
		r.InvalidateCache(userID)
	}
	return result, err
}
