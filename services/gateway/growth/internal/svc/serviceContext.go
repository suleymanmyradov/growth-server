// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package svc

import (
	"fmt"
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/pkg/ai/safety"
	"github.com/suleymanmyradov/growth-server/pkg/auth/jwt"
	"github.com/suleymanmyradov/growth-server/pkg/auth/mdpropagate"
	"github.com/suleymanmyradov/growth-server/pkg/auth/s2s"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/config"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/middleware"
	aicoachrpc "github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/client"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/authservice"
	clientrpc "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/fileManagerClient"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/notificationsClient"
	"github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/searchservice"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config           config.Config
	Auth             rest.Middleware
	RateLimit        rest.Middleware
	TokenMaker       *jwt.TokenMaker
	AuthRpc          authservice.AuthService
	NotificationsRpc notificationsClient.Notifications
	ClientRpc        *clientrpc.Service
	SearchRpc        searchservice.SearchService
	AICoachRpc       *aicoachrpc.Service
	FileManagerRpc   fileManagerClient.FileManager
	// AIClient is the LLM client for the agentic coaching flow. Nil when
	// AI.APIKey is empty — the coaching handler falls back to the legacy
	// ai-coach RPC stream in that case.
	AIClient   ai.Client
	Classifier safety.Classifier
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

	// Base client options with auth propagation, s2s signing, and default timeout.
	// Use zrpc.WithUnaryClientInterceptor so go-zero built-in interceptors
	// (trace, prometheus, breaker, timeout, duration) are still applied.
	baseOpts := []zrpc.ClientOption{
		zrpc.WithUnaryClientInterceptor(mdpropagate.UnaryClientInterceptor()),
		zrpc.WithUnaryClientInterceptor(s2s.UnaryClientInterceptor(s2sCfg)),
		zrpc.WithTimeout(time.Second * 3),
	}

	// AI coach can be slower; give it a longer timeout
	aiCoachOpts := []zrpc.ClientOption{
		zrpc.WithUnaryClientInterceptor(mdpropagate.UnaryClientInterceptor()),
		zrpc.WithUnaryClientInterceptor(s2s.UnaryClientInterceptor(s2sCfg)),
		zrpc.WithTimeout(time.Second * 90),
	}

	// Client RPC handles billing (Stripe API calls) which can be slow due to
	// network latency to Stripe's servers — give it a longer timeout than base.
	clientOpts := []zrpc.ClientOption{
		zrpc.WithUnaryClientInterceptor(mdpropagate.UnaryClientInterceptor()),
		zrpc.WithUnaryClientInterceptor(s2s.UnaryClientInterceptor(s2sCfg)),
		zrpc.WithTimeout(time.Second * 30),
	}

	authRpc := authservice.NewAuthService(zrpc.MustNewClient(c.AuthRpc, baseOpts...))
	clientRpc := clientrpc.NewClientService(zrpc.MustNewClient(c.ClientRpc, clientOpts...))
	aiCoachRpc := aicoachrpc.NewAICoachClientService(
		zrpc.MustNewClient(c.AICoachRpc, aiCoachOpts...),
		zrpc.MustNewClient(c.AICoachRpc, baseOpts...),
	)

	limiters := middleware.BuildRateLimiters(c.RateLimit)

	// AI client for the agentic coaching flow. Optional — if APIKey is
	// empty, the coaching handler falls back to the legacy ai-coach RPC.
	var aiClient ai.Client
	var classifier safety.Classifier
	if c.AI.APIKey != "" {
		client, err := ai.New(c.AI)
		if err != nil {
			logx.Must(fmt.Errorf("failed to create AI client: %w", err))
		}
		aiClient = client
		classifier = safety.NewLLMClassifier(aiClient)
	}

	tokenMaker, err := jwt.NewTokenMaker(jwt.Config{
		Secret:   c.Auth.Secret,
		Issuer:   c.Auth.Issuer,
		Audience: c.Auth.Audience,
	}, nil)
	if err != nil {
		logx.Must(fmt.Errorf("init token maker: %w", err))
	}

	return &ServiceContext{
		Config: c,
		Auth: middleware.JWTMiddleware(middleware.JWTVerifierConfig{
			Secret:   c.Auth.Secret,
			Issuer:   c.Auth.Issuer,
			Audience: c.Auth.Audience,
		}),
		TokenMaker:       tokenMaker,
		RateLimit:        middleware.RateLimitMiddleware(limiters),
		AuthRpc:          authRpc,
		NotificationsRpc: notificationsClient.NewNotifications(zrpc.MustNewClient(c.NotificationsRpc, baseOpts...)),
		ClientRpc:        clientRpc,
		SearchRpc:        searchservice.NewSearchService(zrpc.MustNewClient(c.SearchRpc, baseOpts...)),
		AICoachRpc:       aiCoachRpc,
		FileManagerRpc:   fileManagerClient.NewFileManager(zrpc.MustNewClient(c.FileManagerRpc, baseOpts...)),
		AIClient:         aiClient,
		Classifier:       classifier,
	}
}
