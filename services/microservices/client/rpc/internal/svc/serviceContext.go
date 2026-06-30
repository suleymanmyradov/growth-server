package svc

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/suleymanmyradov/growth-server/pkg/authz"
	"github.com/suleymanmyradov/growth-server/pkg/cache"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/pkg/postgres"
	"github.com/suleymanmyradov/growth-server/pkg/redisutil"
	"github.com/suleymanmyradov/growth-server/pkg/stripe"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/client/aicoachservice"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/analytics"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/fileManagerClient"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/zrpc"
	"golang.org/x/sync/singleflight"
)

type ServiceContext struct {
	Config           config.Config
	Repo             *repository.Repository
	EventsPub        *events.Publisher
	AICoachRpc       aicoachservice.AICoachService
	FileManagerRpc   fileManagerClient.FileManager
	PatternDetection *analytics.PatternDetection
	StripeClient     *stripe.Client
	TxRunner         *postgres.PgxTxRunner
	Authz            *authz.Checker
	// Cache is a Redis-backed read-through cache (with singleflight dedup)
	// used for the assembled personalization context and other hot paths.
	Cache *cache.Cache
	// WeeklyReviewSF dedupes concurrent weekly-review generations for the same
	// (user, week) so a double-submit doesn't run the expensive AI pipeline
	// more than once. The DB unique constraint still guarantees a single row;
	// this only avoids the wasted work and last-write-wins churn.
	WeeklyReviewSF singleflight.Group
	pool           *pgxpool.Pool
	redis          *redis.Client
}

func NewServiceContext(c config.Config) *ServiceContext {
	pool := postgres.MustOpenPool(c.Postgres.Datasource, c.Postgres.MaxOpenConns, c.Postgres.MaxIdleConns, c.Postgres.ConnMaxLifetime)

	queries := db.New(pool)
	txRunner := postgres.NewPgxTxRunner(pool)

	var eventsPub *events.Publisher
	if len(c.Kafka.Brokers) > 0 && c.Kafka.EventsTopic != "" {
		eventsPub = events.NewPublisher(c.Kafka.Brokers, c.Kafka.EventsTopic)
	}

	repo := repository.NewRepository(queries)
	patternDetection := analytics.NewPatternDetection()

	var stripeClient *stripe.Client
	if c.Billing.StripeSecretKey != "" {
		stripeClient = stripe.NewClient(c.Billing.StripeSecretKey)
	}

	aiCoachRpc := aicoachservice.NewAICoachService(zrpc.MustNewClient(c.AICoachRpc, zrpc.WithTimeout(time.Second*90)))
	fileManagerRpc := fileManagerClient.NewFileManager(zrpc.MustNewClient(c.FileManagerRpc))

	var redisClient *redis.Client
	var authzChecker *authz.Checker
	var appCache *cache.Cache
	if c.AppRedis.Addr != "" {
		client, err := redisutil.NewClient(c.AppRedis.Addr, c.AppRedis.Password, c.AppRedis.DB)
		if err != nil {
			logx.Errorf("redis unavailable; authz caching disabled: %v", err)
		} else {
			redisClient = client
			authzChecker = authz.NewChecker(authz.NewRedisCache(redisClient), func(ctx context.Context, userID uuid.UUID) (authz.UserStatus, error) {
				// Use user_settings as a proxy for user existence in the client service.
				// The auth service is the canonical source of truth for user status.
				_, err := repo.UserSettings.GetUserSettings(ctx, userID)
				if err != nil {
					return authz.StatusNotFound, nil
				}
				return authz.StatusActive, nil
			})
		}
	} else {
		logx.Info("Redis not configured; authz caching disabled")
	}

	// Always construct the cache, even with a nil redis client: its methods
	// are no-ops / fetch-through when rdb is nil, so callers (e.g. the
	// personalization context read-through) can use it unconditionally
	// without nil-guarding on every call site.
	appCache = cache.New(redisClient)

	return &ServiceContext{
		Config:           c,
		Repo:             repo,
		EventsPub:        eventsPub,
		AICoachRpc:       aiCoachRpc,
		FileManagerRpc:   fileManagerRpc,
		PatternDetection: patternDetection,
		StripeClient:     stripeClient,
		TxRunner:         txRunner,
		Authz:            authzChecker,
		Cache:            appCache,
		pool:             pool,
		redis:            redisClient,
	}
}

// WithTx returns a new Repository backed by the given transaction.
// Use this inside TxRunner.Run to perform multiple repo operations atomically.
func (s *ServiceContext) WithTx(tx pgx.Tx) *repository.Repository {
	return repository.NewRepository(db.New(tx))
}

// RunInTx executes fn inside a transaction with RLS user context set.
// Use this for any multi-statement write path that must be atomic and
// tenant-isolated (e.g. check-in -> activity -> reminder cancel).
func (s *ServiceContext) RunInTx(ctx context.Context, userID string, fn func(*repository.Repository) error) error {
	return s.TxRunner.Run(ctx, userID, func(tx pgx.Tx) error {
		return fn(s.WithTx(tx))
	})
}

// Pool returns the underlying pgx connection pool.
func (s *ServiceContext) Pool() *pgxpool.Pool {
	return s.pool
}

func (s *ServiceContext) Close() {
	if s.EventsPub != nil {
		_ = s.EventsPub.Close()
	}
	if s.pool != nil {
		s.pool.Close()
	}
	if s.redis != nil {
		_ = s.redis.Close()
	}
}

// PersonalizationContextKey is the Redis cache key for a user's assembled
// personalization context. Shared between the read path (GetPersonalizationContext)
// and the write paths that invalidate it.
func PersonalizationContextKey(userID uuid.UUID) string {
	return personalizationContextKeyPrefix + userID.String()
}

// InvalidatePersonalizationContext drops the cached personalization context for
// a user so the next read rebuilds it from fresh data. Call this from any write
// path that changes data included in the context (check-ins, goals, habits,
// coaching profile, plan-adjustment suggestions). Non-fatal: a stale entry
// simply expires at TTL.
func (s *ServiceContext) InvalidatePersonalizationContext(ctx context.Context, userID uuid.UUID) {
	if s.Cache == nil {
		return
	}
	if err := s.Cache.Delete(ctx, PersonalizationContextKey(userID)); err != nil {
		logx.Errorf("invalidate personalization context for %s: %v", userID, err)
	}
}

const (
	personalizationContextKeyPrefix = "personalization_context:"
	// PersonalizationContextTTL is how long an assembled personalization context
	// is served from cache before being rebuilt. Writes invalidate eagerly, so
	// this only bounds staleness if an invalidation is missed. Tunable.
	PersonalizationContextTTL = 3 * time.Minute
)
