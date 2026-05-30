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
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/consumer/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/consumer/internal/consumer"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/consumer/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/consumer/internal/repository/db"
	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/queue"
)

type ServiceContext struct {
	Config    config.Config
	Repo      *repository.Repository
	AI        ai.Client
	TxRunner  *postgres.PgxTxRunner
	EventsQ   queue.MessageQueue
	EventsPub *events.Publisher
	pool      *pgxpool.Pool
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
	repo := repository.NewRepository(queries)
	txRunner := postgres.NewPgxTxRunner(pool)

	// AI client.
	var aiClient ai.Client
	if c.AI.APIKey != "" {
		client, err := ai.New(c.AI)
		if err != nil {
			logx.Errorf("failed to create AI client: %v", err)
		} else {
			aiClient = client
		}
	}

	// Events publisher (for publishing feedback events).
	var eventsPub *events.Publisher
	if len(c.Kafka.Brokers) > 0 && c.Kafka.EventsTopic != "" {
		eventsPub = events.NewPublisher(c.Kafka.Brokers, c.Kafka.EventsTopic)
	}

	// Create consumer handler.
	handler := consumer.NewEventsHandler(repo, aiClient, eventsPub)

	// Create kq queue.
	eventsQ := kq.MustNewQueue(
		kq.KqConf{
			Brokers: c.Kafka.Brokers,
			Group:   c.Kafka.ConsumerGroup + ".events",
			Topic:   c.Kafka.EventsTopic,
		},
		kq.WithHandle(handler.Consume),
	)

	return &ServiceContext{
		Config:    c,
		Repo:      repo,
		AI:        aiClient,
		TxRunner:  txRunner,
		EventsQ:   eventsQ,
		EventsPub: eventsPub,
		pool:      pool,
	}
}

// WithTx returns a new Repository backed by the given transaction.
func (s *ServiceContext) WithTx(tx pgx.Tx) *repository.Repository {
	return repository.NewRepository(db.NewWithTx(tx))
}

// StartConsumers launches the kq queue.
func (s *ServiceContext) StartConsumers() {
	go s.EventsQ.Start()
	logx.Info("started ai-coach kafka consumer")
}

func (s *ServiceContext) Close() {
	if s.EventsQ != nil {
		s.EventsQ.Stop()
	}
	if s.EventsPub != nil {
		_ = s.EventsPub.Close()
	}
	if s.pool != nil {
		s.pool.Close()
	}
}
