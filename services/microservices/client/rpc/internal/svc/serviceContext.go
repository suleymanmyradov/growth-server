package svc

import (
	"database/sql"

	_ "github.com/lib/pq"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceContext struct {
	Config config.Config
	Repo   *repository.Repository
}

func NewServiceContext(c config.Config) *ServiceContext {
	sqlDB, err := sql.Open("postgres", c.Postgres.Datasource)
	if err != nil {
		logx.Errorf("Failed to open database: %v", err)
		panic(err)
	}

	queries := db.New(sqlDB)

	return &ServiceContext{
		Config: c,
		Repo:   repository.NewRepository(queries),
	}
}
