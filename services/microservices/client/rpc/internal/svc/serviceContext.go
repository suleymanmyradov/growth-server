package svc

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
)

type ServiceContext struct {
	Config    config.Config
	Repo      *repository.Repository
	EventsPub *events.Publisher
	AIClient  ai.Client
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

	var eventsPub *events.Publisher
	if len(c.Kafka.Brokers) > 0 && c.Kafka.EventsTopic != "" {
		eventsPub = events.NewPublisher(c.Kafka.Brokers, c.Kafka.EventsTopic)
	}

	aiClient, err := ai.New(c.AI)
	if err != nil {
		panic(fmt.Errorf("ai client: %w", err))
	}

	return &ServiceContext{
		Config:    c,
		Repo:      repository.NewRepository(queries),
		EventsPub: eventsPub,
		AIClient:  aiClient,
		sqlDB:     sqlDB,
	}
}

func (s *ServiceContext) Close() {
	if s.EventsPub != nil {
		_ = s.EventsPub.Close()
	}
	if s.sqlDB != nil {
		_ = s.sqlDB.Close()
	}
}
