package svc

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/pkg/postgres"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/consumer"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/scheduler"
	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/queue"
)

type ServiceContext struct {
	Config       config.Config
	Repo         *repository.Repository
	EventsPub    *events.Publisher
	Scheduler    *scheduler.Scheduler
	TxRunner     *postgres.PgxTxRunner
	EventsQ      queue.MessageQueue
	ReminderDueQ queue.MessageQueue
	pool         *pgxpool.Pool
	schedCancel  context.CancelFunc
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

	reminderPub := events.NewPublisher(c.Kafka.Brokers, c.Kafka.ReminderDueTopic)

	sched := scheduler.NewScheduler(repo.Reminders, reminderPub, realClock{})

	eventsHandler := consumer.NewEventsHandler(repo, reminderPub, nil)
	reminderDueHandler := consumer.NewReminderDueHandler(repo, nil)

	eventsQ := kq.MustNewQueue(
		kq.KqConf{
			Brokers: c.Kafka.Brokers,
			Group:   c.Kafka.ConsumerGroup + ".events",
			Topic:   c.Kafka.EventsTopic,
		},
		kq.WithHandle(eventsHandler.Consume),
	)

	reminderDueQ := kq.MustNewQueue(
		kq.KqConf{
			Brokers: c.Kafka.Brokers,
			Group:   c.Kafka.ConsumerGroup + ".reminders",
			Topic:   c.Kafka.ReminderDueTopic,
		},
		kq.WithHandle(reminderDueHandler.Consume),
	)

	return &ServiceContext{
		Config:       c,
		Repo:         repo,
		EventsPub:    reminderPub,
		Scheduler:    sched,
		TxRunner:     txRunner,
		EventsQ:      eventsQ,
		ReminderDueQ: reminderDueQ,
		pool:         pool,
	}
}

// WithTx returns a new Repository backed by the given transaction.
func (s *ServiceContext) WithTx(tx pgx.Tx) *repository.Repository {
	return repository.NewRepository(db.New(tx))
}

// StartConsumers launches the scheduler goroutine and both kq queues.
func (s *ServiceContext) StartConsumers() context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())
	s.schedCancel = cancel

	go s.Scheduler.Run(ctx)
	go s.EventsQ.Start()
	go s.ReminderDueQ.Start()

	logx.Info("started scheduler and kafka consumers")
	return cancel
}

func (s *ServiceContext) Close() {
	if s.schedCancel != nil {
		s.schedCancel()
	}
	if s.EventsQ != nil {
		s.EventsQ.Stop()
	}
	if s.ReminderDueQ != nil {
		s.ReminderDueQ.Stop()
	}
	if s.EventsPub != nil {
		_ = s.EventsPub.Close()
	}
	if s.pool != nil {
		s.pool.Close()
	}
}

// realClock implements consumer.Clock and scheduler.Clock.
type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }
