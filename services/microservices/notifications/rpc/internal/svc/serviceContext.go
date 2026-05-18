package svc

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/suleymanmyradov/growth-server/pkg/events"
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
	EventsQ      queue.MessageQueue
	ReminderDueQ queue.MessageQueue
	sqlDB        *sql.DB
	schedCancel  context.CancelFunc
}

func mustOpenDB(datasource string, maxOpen, maxIdle int, maxLifetime time.Duration) *sql.DB {
	db, err := sql.Open("postgres", datasource)
	if err != nil {
		panic(fmt.Errorf("postgres open: %w", err))
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(maxLifetime)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		panic(fmt.Errorf("postgres ping: %w", err))
	}
	return db
}

func NewServiceContext(c config.Config) *ServiceContext {
	sqlDB := mustOpenDB(c.Postgres.Datasource, c.Postgres.MaxOpenConns, c.Postgres.MaxIdleConns, c.Postgres.ConnMaxLifetime)
	queries := db.New(sqlDB)
	repo := repository.NewRepository(queries)

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
		EventsQ:      eventsQ,
		ReminderDueQ: reminderDueQ,
		sqlDB:        sqlDB,
	}
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
	if s.sqlDB != nil {
		_ = s.sqlDB.Close()
	}
}

// realClock implements consumer.Clock and scheduler.Clock.
type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }
