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
	sharedmw "github.com/suleymanmyradov/growth-server/pkg/httpx/middleware"
	"github.com/suleymanmyradov/growth-server/pkg/postgres"
	"github.com/suleymanmyradov/growth-server/pkg/redisutil"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/config"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/mfa"
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
	Config     config.Config
	Auth       rest.Middleware
	AdminAuth  rest.Middleware
	MfaAuth    rest.Middleware
	RateLimit  rest.Middleware
	AuditLog   rest.Middleware
	TokenMaker *jwt.TokenMaker
	// MfaKey is the decoded 32-byte AES-256 key encrypting TOTP secrets at
	// rest; nil when Mfa.EncryptionKey is unset (MFA endpoints then refuse).
	MfaKey            []byte
	ArticlesRpc       clientarticles.Articles
	CategoriesRpc     clientcategories.Categories
	TagsRpc           clienttags.Tags
	ReportRpc         clientreport.Report
	SiteSettingsRpc   clientsitesettings.SiteSettings
	HabitTemplatesRpc clienthabittemplates.HabitTemplates
	GoalTemplatesRpc  clientgoaltemplates.GoalTemplates
	SearchRpc         searchservice.SearchService
	FileManagerRpc    clientfilemanager.FileManager
	AuthRpc           authservice.AuthService
	BillingRpc        clientbilling.BillingService
	EventsPub         *events.Publisher
	Repo              *repository.Repository
	TxRunner          *postgres.PgxTxRunner
	cancel            context.CancelFunc
	pool              *pgxpool.Pool
	auditLogger       *middleware.AuditLogger
}

func NewServiceContext(c config.Config) *ServiceContext {
	// Adminway mints admin tokens (growth-admin audience) — it needs the ES256
	// signing key. There is no symmetric fallback; run `make jwt-keygen-admin`
	// for local development.
	if c.Auth.PrivateKey == "" {
		logx.Must(fmt.Errorf("Auth.PrivateKey is required"))
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

	// Redis-backed token revocation: lets logout invalidate a session so its
	// refresh tokens are rejected. Disabled (with a warning) when Redis is not
	// configured — refresh tokens then stay valid until they expire.
	var revocationRepo jwt.RevocationRepository
	if c.Redis.Addr != "" {
		revocationClient, err := redisutil.NewClient(c.Redis.Addr, c.Redis.Password, c.Redis.DB)
		if err != nil {
			logx.Errorf("redis unavailable; admin token revocation disabled: %v", err)
		} else if revocationRepo, err = jwt.NewRedisRevocationRepository(revocationClient); err != nil {
			logx.Errorf("admin revocation repository init failed: %v", err)
			revocationRepo = nil
		}
	}

	tokenMaker, err := jwt.NewTokenMaker(jwt.Config{
		PrivateKey:            c.Auth.PrivateKey,
		PublicKey:             c.Auth.PublicKey,
		Issuer:                c.Auth.Issuer,
		Audience:              c.Auth.Audience,
		AccessExpiryDuration:  c.Auth.AccessExpiryDuration,
		RefreshExpiryDuration: c.Auth.RefreshExpiryDuration,
	}, revocationRepo)
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
	} else if c.Redis.Addr != "" && c.Kafka.EventsTopic != "" {
		redisClient, err := redisutil.NewClient(c.Redis.Addr, c.Redis.Password, c.Redis.DB)
		if err != nil {
			logx.Errorf("redis unavailable; adminway event publishing disabled: %v", err)
		} else {
			eventsPub = events.NewRedisStreamPublisher(redisClient, c.Kafka.EventsTopic)
		}
	}

	// Admin action audit trail (admin_audit_log). TokenMaker is used as the
	// identity verifier — it resolves which admin a Bearer token belongs to;
	// authorization stays with the route-level Auth/AdminAuth middlewares.
	auditLogger := middleware.NewAuditLogger(queries, tokenMaker)

	// TOTP secret encryption key. Optional so local dev without MFA still
	// boots — but Mfa.Required with no key would lock every admin out, so
	// that combination fails fast here.
	var mfaKey []byte
	if c.Mfa.EncryptionKey != "" {
		var err error
		if mfaKey, err = mfa.ParseKey(c.Mfa.EncryptionKey); err != nil {
			logx.Must(fmt.Errorf("Mfa.EncryptionKey: %w", err))
		}
	} else if c.Mfa.Required {
		logx.Must(fmt.Errorf("Mfa.EncryptionKey is required when Mfa.Required is on"))
	}

	return &ServiceContext{
		Config: c,
		// The TokenMaker doubles as the verifier — it always knows the public
		// half of its own signing key, so admin ES256 tokens verify even when
		// only Auth.PrivateKey is configured.
		Auth:              sharedmw.JWTMiddleware(tokenMaker),
		AdminAuth:         middleware.AdminAuth(),
		MfaAuth:           middleware.MfaAuth(tokenMaker, queries),
		RateLimit:         middleware.RateLimitMiddleware(middleware.BuildRateLimiters(c.RateLimit)),
		AuditLog:          auditLogger.Middleware(),
		TokenMaker:        tokenMaker,
		MfaKey:            mfaKey,
		ArticlesRpc:       clientarticles.NewArticles(zrpc.MustNewClient(c.ClientRpc, baseOpts...)),
		CategoriesRpc:     clientcategories.NewCategories(zrpc.MustNewClient(c.ClientRpc, baseOpts...)),
		TagsRpc:           clienttags.NewTags(zrpc.MustNewClient(c.ClientRpc, baseOpts...)),
		ReportRpc:         clientreport.NewReport(zrpc.MustNewClient(c.ClientRpc, baseOpts...)),
		SiteSettingsRpc:   clientsitesettings.NewSiteSettings(zrpc.MustNewClient(c.ClientRpc, baseOpts...)),
		HabitTemplatesRpc: clienthabittemplates.NewHabitTemplates(zrpc.MustNewClient(c.ClientRpc, baseOpts...)),
		GoalTemplatesRpc:  clientgoaltemplates.NewGoalTemplates(zrpc.MustNewClient(c.ClientRpc, baseOpts...)),
		SearchRpc:         searchservice.NewSearchService(zrpc.MustNewClient(c.SearchRpc, baseOpts...)),
		FileManagerRpc:    clientfilemanager.NewFileManager(zrpc.MustNewClient(c.FileManagerRpc, baseOpts...)),
		AuthRpc:           authservice.NewAuthService(zrpc.MustNewClient(c.AuthRpc, baseOpts...)),
		BillingRpc:        clientbilling.NewBillingService(zrpc.MustNewClient(c.ClientRpc, baseOpts...)),
		EventsPub:         eventsPub,
		Repo:              repo,
		TxRunner:          txRunner,
		cancel:            cancel,
		pool:              pool,
		auditLogger:       auditLogger,
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
	if s.auditLogger != nil {
		s.auditLogger.Close()
	}
	if s.pool != nil {
		s.pool.Close()
	}
}
