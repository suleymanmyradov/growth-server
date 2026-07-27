package svc

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/suleymanmyradov/growth-server/pkg/auth/jwt"
	"github.com/suleymanmyradov/growth-server/pkg/auth/mdpropagate"
	"github.com/suleymanmyradov/growth-server/pkg/auth/s2s"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/pkg/postgres"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/config"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/middleware"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/authservice"
	clientarticles "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/articles"
	clientbilling "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/billingservice"
	clientcategories "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/categories"
	clientgoaltemplates "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/goaltemplates"
	clienthabittemplates "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/habittemplates"
	clientreport "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/report"
	clientsitesettings "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/sitesettings"
	clienttags "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/tags"
	clientfilemanager "github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/fileManagerClient"
	searchservice "github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/searchservice"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config          config.Config
	Auth            rest.Middleware
	AdminAuth       rest.Middleware
	TokenMaker      *jwt.TokenMaker
	ArticlesRpc        clientarticles.Articles
	CategoriesRpc      clientcategories.Categories
	TagsRpc            clienttags.Tags
	ReportRpc          clientreport.Report
	SiteSettingsRpc    clientsitesettings.SiteSettings
	HabitTemplatesRpc  clienthabittemplates.HabitTemplates
	GoalTemplatesRpc   clientgoaltemplates.GoalTemplates
	SearchRpc          searchservice.SearchService
	FileManagerRpc     clientfilemanager.FileManager
	AuthRpc            authservice.AuthService
	BillingRpc         clientbilling.BillingService
	EventsPub       *events.Publisher
	Repo            *repository.Repository
	TxRunner        *postgres.PgxTxRunner
	cancel          context.CancelFunc
	pool            *pgxpool.Pool
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
		zrpc.WithTimeout(time.Second * 3),
	}

	tokenMaker, err := jwt.NewTokenMaker(jwt.Config{
		Secret:                c.Auth.Secret,
		Issuer:                c.Auth.Issuer,
		Audience:              c.Auth.Audience,
		AccessExpiryDuration:  c.Auth.AccessExpiryDuration,
		RefreshExpiryDuration: c.Auth.RefreshExpiryDuration,
	}, nil)
	if err != nil {
		logx.Must(fmt.Errorf("init token maker: %w", err))
	}

	pool := postgres.MustOpenPool(c.Postgres.Datasource, c.Postgres.MaxOpenConns, c.Postgres.MaxIdleConns, c.Postgres.ConnMaxLifetime)
	queries := db.New(pool)
	repo := repository.NewRepository(queries)
	txRunner := postgres.NewPgxTxRunner(pool)

	cancel := func() {}

	var eventsPub *events.Publisher
	if len(c.Kafka.Brokers) > 0 && c.Kafka.EventsTopic != "" {
		eventsPub = events.NewPublisher(c.Kafka.Brokers, c.Kafka.EventsTopic)
	}

	return &ServiceContext{
		Config: c,
		Auth: middleware.JWTMiddleware(middleware.JWTVerifierConfig{
			Secret:   c.Auth.Secret,
			Issuer:   c.Auth.Issuer,
			Audience: c.Auth.Audience,
		}),
		AdminAuth:       middleware.AdminAuth(),
		TokenMaker:      tokenMaker,
		ArticlesRpc:       clientarticles.NewArticles(zrpc.MustNewClient(c.ClientRpc, baseOpts...)),
		CategoriesRpc:     clientcategories.NewCategories(zrpc.MustNewClient(c.ClientRpc, baseOpts...)),
		TagsRpc:           clienttags.NewTags(zrpc.MustNewClient(c.ClientRpc, baseOpts...)),
		ReportRpc:         clientreport.NewReport(zrpc.MustNewClient(c.ClientRpc, baseOpts...)),
		SiteSettingsRpc:   clientsitesettings.NewSiteSettings(zrpc.MustNewClient(c.ClientRpc, baseOpts...)),
		HabitTemplatesRpc: clienthabittemplates.NewHabitTemplates(zrpc.MustNewClient(c.ClientRpc, baseOpts...)),
		GoalTemplatesRpc:  clientgoaltemplates.NewGoalTemplates(zrpc.MustNewClient(c.ClientRpc, baseOpts...)),
		SearchRpc:       searchservice.NewSearchService(zrpc.MustNewClient(c.SearchRpc, baseOpts...)),
		FileManagerRpc:  clientfilemanager.NewFileManager(zrpc.MustNewClient(c.FileManagerRpc, baseOpts...)),
		AuthRpc:         authservice.NewAuthService(zrpc.MustNewClient(c.AuthRpc, baseOpts...)),
		BillingRpc:      clientbilling.NewBillingService(zrpc.MustNewClient(c.ClientRpc, baseOpts...)),
		EventsPub:       eventsPub,
		Repo:            repo,
		TxRunner:        txRunner,
		cancel:          cancel,
		pool:            pool,
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
	if s.pool != nil {
		s.pool.Close()
	}
}
