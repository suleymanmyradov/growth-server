package svc

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/suleymanmyradov/growth-server/pkg/auth/jwt"
	"github.com/suleymanmyradov/growth-server/pkg/email"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/pkg/postgres"
	"github.com/suleymanmyradov/growth-server/pkg/redisutil"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/logx"
)

// TokenMakerInterface defines the token operations used by the auth logic.
// *jwt.TokenMaker satisfies this interface; tests can provide mock implementations.
type TokenMakerInterface interface {
	CreateAccessToken(ctx context.Context, userID uuid.UUID, username string, roles []string, sessionID uuid.UUID) (*jwt.TokenResponse, error)
	CreateRefreshToken(ctx context.Context, userID uuid.UUID, username string, roles []string, sessionID uuid.UUID) (*jwt.TokenResponse, error)
	VerifyAccessToken(ctx context.Context, tokenString string) (*jwt.TokenClaims, error)
	VerifyRefreshToken(ctx context.Context, tokenString string) (*jwt.TokenClaims, error)
	RevokeAccessToken(ctx context.Context, tokenString string) error
	RevokeRefreshToken(ctx context.Context, tokenString string) error
	RevokeSession(ctx context.Context, sessionID uuid.UUID, ttl time.Duration) error
	IsSessionRevoked(ctx context.Context, sessionID uuid.UUID) (bool, error)
	RotateRefreshToken(ctx context.Context, oldToken string) (*jwt.TokenResponse, error)
}

// TxRunnerInterface defines the transaction runner used by the auth logic.
// *postgres.PgxTxRunner satisfies this interface; tests can provide mock implementations.
type TxRunnerInterface interface {
	Run(ctx context.Context, userID string, fn func(pgx.Tx) error) error
}

type ServiceContext struct {
	Config       config.Config
	Repo         *repository.Repository
	TokenMaker   TokenMakerInterface
	TxRunner     TxRunnerInterface
	RedisClient  *redis.Client
	EmailSender  email.Sender
	EventsPub    *events.Publisher
	cancel       context.CancelFunc
	pool         *pgxpool.Pool
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

	emailSender, err := email.New(email.Config{
		Provider:    c.Email.Provider,
		APIKey:      c.Email.APIKey,
		FromAddress: c.Email.FromAddress,
	})
	if err != nil {
		logx.Must(err)
	}

	var eventsPub *events.Publisher
	if len(c.Kafka.Brokers) > 0 && c.Kafka.EventsTopic != "" {
		eventsPub = events.NewPublisher(c.Kafka.Brokers, c.Kafka.EventsTopic)
	}

	return &ServiceContext{
		Config:      c,
		Repo:        repo,
		TokenMaker:  tokenMaker,
		TxRunner:    txRunner,
		RedisClient: redisClient,
		EmailSender: emailSender,
		EventsPub:   eventsPub,
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
	if s.EventsPub != nil {
		_ = s.EventsPub.Close()
	}
	if s.RedisClient != nil {
		_ = s.RedisClient.Close()
	}
	if s.pool != nil {
		s.pool.Close()
	}
}
