package svc

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	expo "github.com/suleymanmyradov/growth-server/pkg/notifications/expo"
	"github.com/suleymanmyradov/growth-server/pkg/postgres"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/consumer"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/delivery"
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
	// PushSender delivers push notifications via Expo. Nil-safe: when Expo is
	// disabled in config, Send is a no-op. See docs/push-notifications-design.md.
	PushSender  *delivery.Sender
	// ReceiptWorker checks Expo push receipts asynchronously and disables
	// stale tokens. Nil-safe: when Expo is disabled, Run is a no-op.
	ReceiptWorker *delivery.ReceiptWorker
	pool          *pgxpool.Pool
	schedCancel   context.CancelFunc
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
	dlqPub := events.NewDLQPublisher(c.Kafka.Brokers, events.DLQTopic)

	sched := scheduler.NewScheduler(repo.Reminders, reminderPub, realClock{})

	// Expo push client + delivery sender. When Expo is disabled in config,
	// the sender is still constructed (with a nil Expo client) so callers can
	// invoke Send without nil-checks; it is a no-op in that case.
	var expoClient *expo.Client
	if c.Expo.Enabled {
		expoClient = expo.NewClient(nil, c.Expo.AccessToken)
	}
	pushSender := delivery.NewSender(repo.Devices, repo.PushTickets, expoClient, c.Expo.Enabled)
	receiptWorker := delivery.NewReceiptWorker(repo.Devices, repo.PushTickets, expoClient)

	eventsHandler := consumer.NewEventsHandler(repo, reminderPub, nil, txRunner, dlqPub)
	reminderDueHandler := consumer.NewReminderDueHandler(repo, nil, txRunner, dlqPub, pushSender)

	// Consumers/Processors must be set explicitly: their `default=8` tags only
	// apply when the KqConf is loaded via conf.Load, not for struct literals.
	// With zero values kq starts no goroutines and the consumer exits immediately.
	eventsQ := kq.MustNewQueue(
		kq.KqConf{
			Brokers:    c.Kafka.Brokers,
			Group:      c.Kafka.ConsumerGroup + ".events",
			Topic:      c.Kafka.EventsTopic,
			Offset:     "first",
			Consumers:  8,
			Processors: 8,
		},
		kq.WithHandle(eventsHandler.Consume),
	)

	reminderDueQ := kq.MustNewQueue(
		kq.KqConf{
			Brokers:    c.Kafka.Brokers,
			Group:      c.Kafka.ConsumerGroup + ".reminders",
			Topic:      c.Kafka.ReminderDueTopic,
			Offset:     "first",
			Consumers:  8,
			Processors: 8,
		},
		kq.WithHandle(reminderDueHandler.Consume),
	)

	return &ServiceContext{
		Config:        c,
		Repo:          repo,
		EventsPub:     reminderPub,
		Scheduler:     sched,
		TxRunner:      txRunner,
		EventsQ:       eventsQ,
		ReminderDueQ:  reminderDueQ,
		PushSender:    pushSender,
		ReceiptWorker: receiptWorker,
		pool:          pool,
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
	if s.ReceiptWorker != nil {
		go s.ReceiptWorker.Run(ctx)
	}

	logx.Info("started scheduler, kafka consumers, and push receipt worker")
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
