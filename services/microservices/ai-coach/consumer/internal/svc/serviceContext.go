package svc

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/pkg/events"
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
	EventsQ   queue.MessageQueue
	EventsPub *events.Publisher
	sqlDB     *sql.DB
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
		EventsQ:   eventsQ,
		EventsPub: eventsPub,
		sqlDB:     sqlDB,
	}
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
	if s.sqlDB != nil {
		_ = s.sqlDB.Close()
	}
}
