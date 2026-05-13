package svc

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceContext struct {
	Config config.Config
	Repo   *repository.Repository
	AI     ai.Client
	sqlDB  *sql.DB
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

	var aiClient ai.Client
	if c.AI.APIKey != "" {
		client, err := ai.New(c.AI)
		if err != nil {
			logx.Errorf("failed to create AI client: %v", err)
		} else {
			aiClient = client
		}
	}

	return &ServiceContext{
		Config: c,
		Repo:   repository.NewRepository(queries),
		AI:     aiClient,
		sqlDB:  sqlDB,
	}
}

func (s *ServiceContext) Close() {
	if s.sqlDB != nil {
		_ = s.sqlDB.Close()
	}
}
