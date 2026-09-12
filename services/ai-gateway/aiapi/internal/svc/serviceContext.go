// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package svc

import (
	"context"
	"fmt"
	"sync"
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
	clientbilling "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/billingservice"
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
	// QuotaStore is the shared Redis quota counter store (same counters the
	// AI clients record usage into). Nil when AI.quota.redis_addr is unset —
	// edge quota checks then pass through and only the internal pkg/ai
	// backstop applies.
	QuotaStore ai.QuotaStore
	// planCapCache caches per-user plan→cap decisions for planCapCacheTTL so
	// bursty AI traffic does not hammer the client billing RPC.
	planCapCache sync.Map
}

// planCapCacheTTL is how long a resolved per-user token cap is reused.
const planCapCacheTTL = time.Minute

type planCapEntry struct {
	cap       int64
	expiresAt time.Time
}

// DailyTokenCap resolves the per-user daily AI token cap from the user's
// plan: an active Pro subscription gets the configured default cap, everyone
// else gets FreeUserDailyTokenCap. When plan-aware caps are not configured
// (free cap unset) or the billing lookup fails, the default cap applies —
// the global daily cost cap still bounds total platform spend.
func (s *ServiceContext) DailyTokenCap(ctx context.Context, userID string) int64 {
	defaultCap := s.Config.AI.Quota.UserDailyTokenCap
	freeCap := s.Config.AI.Quota.FreeUserDailyTokenCap
	if freeCap <= 0 {
		return defaultCap
	}
	if entry, ok := s.planCapCache.Load(userID); ok {
		if e, ok := entry.(planCapEntry); ok && time.Now().Before(e.expiresAt) {
			return e.cap
		}
	}

	cap := freeCap
	resp, err := s.ClientRpc.BillingService.GetBillingOverview(ctx, &clientbilling.GetBillingOverviewRequest{})
	if err != nil {
		logx.WithContext(ctx).Errorf("ai quota: billing lookup failed for user %s, applying free cap: %v", userID, err)
	} else if ent := resp.GetEntitlements(); ent != nil && ent.GetPlanCode() == "pro" {
		switch ent.GetStatus() {
		case "active", "trialing", "past_due":
			cap = defaultCap
		}
	}

	s.planCapCache.Store(userID, planCapEntry{cap: cap, expiresAt: time.Now().Add(planCapCacheTTL)})
	return cap
}

// CheckDailyTokenQuota enforces the plan-aware daily token cap at the edge,
// before any AI work is dispatched. Usage is recorded into the same Redis
// counters by the AI clients, so this reads the shared counter. Best-effort:
// if the quota store is unavailable the check passes — the fail-closed
// per-call check inside pkg/ai still applies.
func (s *ServiceContext) CheckDailyTokenQuota(ctx context.Context, userID string) error {
	if s.QuotaStore == nil {
		return nil
	}
	cap := s.DailyTokenCap(ctx, userID)
	if cap <= 0 {
		return nil
	}
	ok, err := s.QuotaStore.CheckUserQuota(ctx, userID, cap)
	if err != nil {
		logx.WithContext(ctx).Errorf("ai quota: edge check error for user %s: %v", userID, err)
		return nil
	}
	if !ok {
		return ai.ErrQuotaExceeded
	}
	return nil
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
	// honour the per-user token cap and global daily cost cap. The same
	// store is kept on the ServiceContext for edge quota checks.
	var quotaStore ai.QuotaStore
	aiOpts := []ai.Option{}
	if c.AI.Quota.RedisAddr != "" {
		quotaRedis, err := redisutil.NewClient(c.AI.Quota.RedisAddr, c.AI.Quota.RedisPassword, c.AI.Quota.RedisDB)
		if err == nil {
			quotaStore = ai.NewRedisQuotaStore(quotaRedis)
			aiOpts = append(aiOpts, ai.WithQuotaStore(quotaStore))
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
		QuotaStore: quotaStore,
	}
}
