package svc

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/pkg/postgres"
	"github.com/suleymanmyradov/growth-server/pkg/stripe"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/analytics"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
)

type ServiceContext struct {
	Config           config.Config
	Repo             *repository.Repository
	EventsPub        *events.Publisher
	AIClient         ai.Client
	PatternDetection *analytics.PatternDetection
	StripeClient     *stripe.Client
	TxRunner         *postgres.PgxTxRunner
	pool             *pgxpool.Pool
}

func mustOpenDB(datasource string, maxOpen, maxIdle int, maxLifetime time.Duration) *pgxpool.Pool {
	config, err := pgxpool.ParseConfig(datasource)
	if err != nil {
		panic(fmt.Errorf("parse pgx config: %w", err))
	}
	config.MaxConns = int32(maxOpen)
	config.MinConns = int32(maxIdle)
	config.MaxConnLifetime = maxLifetime

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		panic(fmt.Errorf("pgx pool: %w", err))
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		panic(fmt.Errorf("pgx ping: %w", err))
	}
	return pool
}

func NewServiceContext(c config.Config) *ServiceContext {
	pool := mustOpenDB(c.Postgres.Datasource, c.Postgres.MaxOpenConns, c.Postgres.MaxIdleConns, c.Postgres.ConnMaxLifetime)

	queries := db.New(pool)
	txRunner := postgres.NewPgxTxRunner(pool)

	var eventsPub *events.Publisher
	if len(c.Kafka.Brokers) > 0 && c.Kafka.EventsTopic != "" {
		eventsPub = events.NewPublisher(c.Kafka.Brokers, c.Kafka.EventsTopic)
	}

	aiClient, err := ai.New(c.AI)
	if err != nil {
		panic(fmt.Errorf("ai client: %w", err))
	}

	repo := repository.NewRepository(queries)
	patternDetection := analytics.NewPatternDetection()

	var stripeClient *stripe.Client
	if c.Billing.StripeSecretKey != "" {
		stripeClient = stripe.NewClient(c.Billing.StripeSecretKey)
	}

	return &ServiceContext{
		Config:           c,
		Repo:             repo,
		EventsPub:        eventsPub,
		AIClient:         aiClient,
		PatternDetection: patternDetection,
		StripeClient:     stripeClient,
		TxRunner:         txRunner,
		pool:             pool,
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
}
