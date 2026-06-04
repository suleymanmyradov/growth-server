package svc

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/suleymanmyradov/growth-server/pkg/postgres"
	"github.com/suleymanmyradov/growth-server/services/microservices/billing-reconciler/internal/config"
)

type ServiceContext struct {
	Config config.Config
	Pool   *pgxpool.Pool
}

func NewServiceContext(c config.Config) *ServiceContext {
	pool := postgres.MustOpenPool(
		c.Postgres.Datasource,
		c.Postgres.MaxOpenConns,
		c.Postgres.MaxIdleConns,
		c.Postgres.ConnMaxLifetime,
	)

	return &ServiceContext{
		Config: c,
		Pool:   pool,
	}
}

func (s *ServiceContext) Close() {
	if s.Pool != nil {
		s.Pool.Close()
	}
}
