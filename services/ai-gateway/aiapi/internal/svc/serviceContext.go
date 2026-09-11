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
	sharedmw "github.com/suleymanmyradov/growth-server/pkg/httpx/middleware"
	"github.com/suleymanmyradov/growth-server/pkg/redisutil"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/config"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/middleware"
	aicoachrpc "github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/client"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/authservice"
	clientrpc "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client"
	searchservice "github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/searchservice"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config     config.Config
	Auth       rest.Middleware
	RateLimit  rest.Middleware
	TokenMaker *jwt.TokenMaker
	AuthRpc    authservice.AuthService
	ClientRpc  *clientrpc.Service
	AICoachRpc *aicoachrpc.Service
	// SearchRpc is the search microservice client, used by the article
	// reference coaching tool. Optional — nil when SearchRpc is not
	// configured, in which case the search_articles tool returns an error
	// and the coach falls back to a text-only reply.
	SearchRpc searchservice.SearchService
	// AIClient is the LLM client for the agentic coaching flow.
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
	if c.AI.APIKey == "" {
		logx.Must(fmt.Errorf("AI.APIKey is required — the agentic coaching path is the only path served by ai-gateway"))
	}

	s2sCfg := s2s.Config{Secret: c.ServiceAuth.Secret}

	// Base client options with auth propagation, s2s signing, and default timeout.
	baseOpts := []zrpc.ClientOption{
		zrpc.WithUnaryClientInterceptor(mdpropagate.UnaryClientInterceptor()),
		zrpc.WithUnaryClientInterceptor(s2s.UnaryClientInterceptor(s2sCfg)),
		zrpc.WithTimeout(time.Second * 3),
	}

	// AI coach can be slower; give it a longer timeout.
	aiCoachOpts := []zrpc.ClientOption{
		zrpc.WithUnaryClientInterceptor(mdpropagate.UnaryClientInterceptor()),
		zrpc.WithUnaryClientInterceptor(s2s.UnaryClientInterceptor(s2sCfg)),
		zrpc.WithTimeout(time.Second * 90),
	}

	// Client RPC handles personalization context which can be slow.
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

	limiters := sharedmw.BuildRateLimiters(c.RateLimit)

	// Quota store (optional). Wired when AI.quota.redis_addr is set; the
	// agentic coaching path is the most expensive AI surface, so it must
	// honour the per-user token cap and global daily cost cap.
	aiOpts := []ai.Option{}
	if c.AI.Quota.RedisAddr != "" {
		quotaRedis, err := redisutil.NewClient(c.AI.Quota.RedisAddr, c.AI.Quota.RedisPassword, c.AI.Quota.RedisDB)
		if err == nil {
			aiOpts = append(aiOpts, ai.WithQuotaStore(ai.NewRedisQuotaStore(quotaRedis)))
		} else {
			logx.Errorf("redis unavailable; AI quotas disabled: %v", err)
		}
	}
	aiClient, err := ai.New(c.AI, aiOpts...)
	if err != nil {
		logx.Must(fmt.Errorf("failed to create AI client: %w", err))
	}
	classifier := safety.NewLLMClassifier(aiClient)

	tokenMaker, err := jwt.NewTokenMaker(jwt.Config{
		Secret:   c.Auth.Secret,
		Issuer:   c.Auth.Issuer,
		Audience: c.Auth.Audience,
	}, nil)
	if err != nil {
		logx.Must(fmt.Errorf("init token maker: %w", err))
	}

	// Search RPC (optional). Used by the article reference coaching tool.
	// Nil when SearchRpc.Endpoints is empty — the search_articles tool then
	// returns an error and the coach falls back to a text-only reply.
	var searchRpc searchservice.SearchService
	if len(c.SearchRpc.Endpoints) > 0 {
		searchRpc = searchservice.NewSearchService(zrpc.MustNewClient(c.SearchRpc, baseOpts...))
	}

	return &ServiceContext{
		Config: c,
		Auth: sharedmw.JWTMiddleware(sharedmw.JWTVerifierConfig{
			Secret:   c.Auth.Secret,
			Issuer:   c.Auth.Issuer,
			Audience: c.Auth.Audience,
		}),
		TokenMaker: tokenMaker,
		RateLimit:  middleware.RateLimitMiddleware(limiters),
		AuthRpc:    authRpc,
		ClientRpc:  clientRpc,
		AICoachRpc: aiCoachRpc,
		SearchRpc:  searchRpc,
		AIClient:   aiClient,
		Classifier: classifier,
	}
}
