package svc

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
	"github.com/suleymanmyradov/growth-server/pkg/auth/jwt"
	"github.com/suleymanmyradov/growth-server/pkg/postgres"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceContext struct {
	Config     config.Config
	Repo       *repository.Repository
	TokenMaker *jwt.TokenMaker
	TxRunner   *postgres.PgxTxRunner
	cancel     context.CancelFunc
	pool       *pgxpool.Pool
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

func mustNewRedis(addr, password string, db int) *goredis.Client {
	c := goredis.NewClient(&goredis.Options{Addr: addr, Password: password, DB: db})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := c.Ping(ctx).Err(); err != nil {
		_ = c.Close()
		panic(fmt.Errorf("redis ping: %w", err))
	}
	return c
}

func NewServiceContext(c config.Config) *ServiceContext {
	pool := mustOpenDB(c.Postgres.Datasource, c.Postgres.MaxOpenConns, c.Postgres.MaxIdleConns, c.Postgres.ConnMaxLifetime)

	queries := db.New(pool)
	repo := repository.NewRepository(queries)
	txRunner := postgres.NewPgxTxRunner(pool)

	redisClient := mustNewRedis(c.Cache.Redis.Addr, c.Cache.Redis.Password, c.Cache.Redis.DB)

	if c.JWT.Secret == "" {
		logx.Must(fmt.Errorf("JWT.Secret is required"))
	}

	tokenRepo, err := repository.NewCmdableRedisRepository(redisClient)
	if err != nil {
		logx.Must(err)
	}

	cancel := func() {}
	tokenConfig := jwt.Config{
		Secret:                c.JWT.Secret,
		Issuer:                c.JWT.Issuer,
		Audience:              c.JWT.Audience,
		AccessExpiryDuration:  c.JWT.AccessExpiryDuration,
		RefreshExpiryDuration: c.JWT.RefreshExpiryDuration,
	}

	tokenMaker, err := jwt.NewTokenMaker(tokenConfig, tokenRepo)
	if err != nil {
		logx.Must(err)
	}

	return &ServiceContext{
		Config:     c,
		Repo:       repo,
		TokenMaker: tokenMaker,
		TxRunner:   txRunner,
		cancel:     cancel,
		pool:       pool,
	}
}

func (s *ServiceContext) Pool() *pgxpool.Pool {
	return s.pool
}

func (s *ServiceContext) Close() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.pool != nil {
		s.pool.Close()
	}
}
