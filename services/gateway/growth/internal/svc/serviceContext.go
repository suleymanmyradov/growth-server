// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package svc

import (
	"fmt"
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/auth/jwt"
	"github.com/suleymanmyradov/growth-server/pkg/auth/mdpropagate"
	"github.com/suleymanmyradov/growth-server/pkg/auth/s2s"
	"github.com/suleymanmyradov/growth-server/pkg/stripe"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/config"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/middleware"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/aicoachservice"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/authservice"
	clientactivity "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/activity"
	clientarticles "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/articles"
	clientbilling "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/billingservice"
	clientcategories "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/categories"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/checkinservice"
	clientgoals "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/goals"
	clienthabits "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/habits"
	clientpersonalization "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/personalizationservice"
	clientreport "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/report"
	clientsaved "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/saved"
	clientsettings "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/settings"
	clientweeklyreview "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/weeklyreviewservice"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/fileManagerClient"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/notificationsClient"
	"github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/searchservice"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config             config.Config
	Auth               rest.Middleware
	RateLimit          rest.Middleware
	TokenMaker         *jwt.TokenMaker
	AuthRpc            authservice.AuthService
	NotificationsRpc   notificationsClient.Notifications
	SavedRpc           clientsaved.Saved
	SettingsRpc        clientsettings.Settings
	ReportRpc          clientreport.Report
	SearchRpc          searchservice.SearchService
	HabitsRpc          clienthabits.Habits
	GoalsRpc           clientgoals.Goals
	CategoriesRpc      clientcategories.Categories
	ArticlesRpc        clientarticles.Articles
	CheckInRpc         checkinservice.CheckInService
	ActivityRpc        clientactivity.Activity
	WeeklyReviewRpc    clientweeklyreview.WeeklyReviewService
	PersonalizationRpc clientpersonalization.PersonalizationService
	BillingRpc         clientbilling.BillingService
	AICoachRpc         aicoachservice.AICoachService
	FileManagerRpc     fileManagerClient.FileManager
	StripeClient       *stripe.Client
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
		zrpc.WithTimeout(time.Second * 15),
	}

	authRpc := authservice.NewAuthService(zrpc.MustNewClient(c.AuthRpc, baseOpts...))
	var stripeClient *stripe.Client
	if c.Billing.StripeSecretKey != "" {
		stripeClient = stripe.NewClient(c.Billing.StripeSecretKey)
	}

	limiters := middleware.BuildRateLimiters(c.RateLimit)

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
		TokenMaker:         tokenMaker,
		RateLimit:          middleware.RateLimitMiddleware(limiters),
		AuthRpc:            authRpc,
		NotificationsRpc:   notificationsClient.NewNotifications(zrpc.MustNewClient(c.NotificationsRpc, baseOpts...)),
		SavedRpc:           clientsaved.NewSaved(zrpc.MustNewClient(c.ClientRpc, baseOpts...)),
		SettingsRpc:        clientsettings.NewSettings(zrpc.MustNewClient(c.ClientRpc, baseOpts...)),
		ReportRpc:          clientreport.NewReport(zrpc.MustNewClient(c.ClientRpc, baseOpts...)),
		SearchRpc:          searchservice.NewSearchService(zrpc.MustNewClient(c.SearchRpc, baseOpts...)),
		HabitsRpc:          clienthabits.NewHabits(zrpc.MustNewClient(c.ClientRpc, baseOpts...)),
		GoalsRpc:           clientgoals.NewGoals(zrpc.MustNewClient(c.ClientRpc, baseOpts...)),
		CategoriesRpc:      clientcategories.NewCategories(zrpc.MustNewClient(c.ClientRpc, baseOpts...)),
		ArticlesRpc:        clientarticles.NewArticles(zrpc.MustNewClient(c.ClientRpc, baseOpts...)),
		CheckInRpc:         checkinservice.NewCheckInService(zrpc.MustNewClient(c.ClientRpc, baseOpts...)),
		ActivityRpc:        clientactivity.NewActivity(zrpc.MustNewClient(c.ClientRpc, baseOpts...)),
		WeeklyReviewRpc:    clientweeklyreview.NewWeeklyReviewService(zrpc.MustNewClient(c.ClientRpc, aiCoachOpts...)),
		PersonalizationRpc: clientpersonalization.NewPersonalizationService(zrpc.MustNewClient(c.ClientRpc, aiCoachOpts...)),
		BillingRpc:         clientbilling.NewBillingService(zrpc.MustNewClient(c.ClientRpc, baseOpts...)),
		AICoachRpc:         aicoachservice.NewAICoachService(zrpc.MustNewClient(c.AICoachRpc, aiCoachOpts...)),
		FileManagerRpc:     fileManagerClient.NewFileManager(zrpc.MustNewClient(c.FileManagerRpc, baseOpts...)),
		StripeClient:       stripeClient,
	}
}
