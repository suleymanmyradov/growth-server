package svc

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/suleymanmyradov/growth-server/pkg/auth/jwt"
	"github.com/suleymanmyradov/growth-server/pkg/postgres"
	"github.com/suleymanmyradov/growth-server/pkg/redisutil"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceContext struct {
	Config      config.Config
	Repo        *repository.Repository
	TokenMaker  *jwt.TokenMaker
	TxRunner    *postgres.PgxTxRunner
	RedisClient *redis.Client
	cancel      context.CancelFunc
	pool        *pgxpool.Pool
}

func NewServiceContext(c config.Config) *ServiceContext {
	pool := postgres.MustOpenPool(c.Postgres.Datasource, c.Postgres.MaxOpenConns, c.Postgres.MaxIdleConns, c.Postgres.ConnMaxLifetime)

	queries := db.New(pool)
	repo := repository.NewRepository(queries)
	txRunner := postgres.NewPgxTxRunner(pool)

	var tokenRepo jwt.RevocationRepository
	var redisClient *redis.Client
	if c.Cache.Redis.Addr != "" {
		client, err := redisutil.NewClient(c.Cache.Redis.Addr, c.Cache.Redis.Password, c.Cache.Redis.DB)
		if err != nil {
			logx.Errorf("redis unavailable; token revocation disabled: %v", err)
		} else {
			redisClient = client
			tokenRepo, err = repository.NewCmdableRedisRepository(client)
			if err != nil {
				logx.Errorf("redis revocation repository init failed: %v", err)
				tokenRepo = nil
			}
		}
	}

	if c.JWT.Secret == "" {
		logx.Must(fmt.Errorf("JWT.Secret is required"))
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
		Config:      c,
		Repo:        repo,
		TokenMaker:  tokenMaker,
		TxRunner:    txRunner,
		RedisClient: redisClient,
		cancel:      cancel,
		pool:        pool,
	}
}

func (s *ServiceContext) Pool() *pgxpool.Pool {
	return s.pool
}

func (s *ServiceContext) Close() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.RedisClient != nil {
		_ = s.RedisClient.Close()
	}
	if s.pool != nil {
		s.pool.Close()
	}
}
