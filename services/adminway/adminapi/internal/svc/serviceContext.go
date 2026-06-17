package svc

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/suleymanmyradov/growth-server/pkg/auth/jwt"
	"github.com/suleymanmyradov/growth-server/pkg/auth/mdpropagate"
	"github.com/suleymanmyradov/growth-server/pkg/auth/s2s"
	"github.com/suleymanmyradov/growth-server/pkg/postgres"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/config"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/middleware"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/repository/db"
	clientarticles "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/articles"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/fileManagerClient"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config         config.Config
	Auth           rest.Middleware
	AdminAuth      rest.Middleware
	TokenMaker     *jwt.TokenMaker
	ArticlesRpc    clientarticles.Articles
	FileManagerRpc fileManagerClient.FileManager
	Repo           *repository.Repository
	TxRunner       *postgres.PgxTxRunner
	cancel         context.CancelFunc
	pool           *pgxpool.Pool
}

func NewServiceContext(c config.Config) *ServiceContext {
	if c.Auth.Secret == "" {
		logx.Must(fmt.Errorf("Auth.Secret is required"))
	}
	if c.Auth.Issuer == "" {
		logx.Must(fmt.Errorf("Auth.Issuer is required"))
	}
	if c.Auth.Audience == "" {
		logx.Must(fmt.Errorf("Auth.Audience is required"))
	}

	s2sCfg := s2s.Config{Secret: c.ServiceAuth.Secret}

	baseOpts := []zrpc.ClientOption{
		zrpc.WithUnaryClientInterceptor(mdpropagate.UnaryClientInterceptor()),
		zrpc.WithUnaryClientInterceptor(s2s.UnaryClientInterceptor(s2sCfg)),
		zrpc.WithTimeout(3),
	}

	tokenMaker, err := jwt.NewTokenMaker(jwt.Config{
		Secret:   c.Auth.Secret,
		Issuer:   c.Auth.Issuer,
		Audience: c.Auth.Audience,
	}, nil)
	if err != nil {
		logx.Must(fmt.Errorf("init token maker: %w", err))
	}

	pool := postgres.MustOpenPool(c.Postgres.Datasource, c.Postgres.MaxOpenConns, c.Postgres.MaxIdleConns, c.Postgres.ConnMaxLifetime)
	queries := db.New(pool)
	repo := repository.NewRepository(queries)
	txRunner := postgres.NewPgxTxRunner(pool)

	cancel := func() {}

	return &ServiceContext{
		Config: c,
		Auth: middleware.JWTMiddleware(middleware.JWTVerifierConfig{
			Secret:   c.Auth.Secret,
			Issuer:   c.Auth.Issuer,
			Audience: c.Auth.Audience,
		}),
		AdminAuth:      middleware.AdminAuth(),
		TokenMaker:     tokenMaker,
		ArticlesRpc:    clientarticles.NewArticles(zrpc.MustNewClient(c.ClientRpc, baseOpts...)),
		FileManagerRpc: fileManagerClient.NewFileManager(zrpc.MustNewClient(c.FileManagerRpc, baseOpts...)),
		Repo:           repo,
		TxRunner:       txRunner,
		cancel:         cancel,
		pool:           pool,
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
