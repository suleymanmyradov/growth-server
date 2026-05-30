package svc

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
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
	TxRunner   *postgres.TxRunner
	cancel     context.CancelFunc
	sqlDB      *sql.DB
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
	sqlDB := mustOpenDB(c.Postgres.Datasource, c.Postgres.MaxOpenConns, c.Postgres.MaxIdleConns, c.Postgres.ConnMaxLifetime)

	queries := db.New(sqlDB)
	repo := repository.NewRepository(queries)
	txRunner := postgres.NewTxRunner(sqlDB)

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
		sqlDB:      sqlDB,
	}
}

func (s *ServiceContext) Close() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.sqlDB != nil {
		_ = s.sqlDB.Close()
	}
}
