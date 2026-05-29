// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package svc

import (
	"fmt"

	"github.com/suleymanmyradov/growth-server/pkg/auth/mdpropagate"
	"github.com/suleymanmyradov/growth-server/pkg/stripe"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/config"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/middleware"
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
	conversationsservice "github.com/suleymanmyradov/growth-server/services/microservices/conversations/rpc/client/conversationsservice"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/notificationsClient"
	searchservice "github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/searchservice"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
)

type ServiceContext struct {
	Config             config.Config
	Auth               rest.Middleware
	AuthRpc            authservice.AuthService
	NotificationsRpc   notificationsClient.Notifications
	SavedRpc           clientsaved.Saved
	SettingsRpc        clientsettings.Settings
	ReportRpc          clientreport.Report
	SearchRpc          searchservice.SearchService
	ConversationsRpc   conversationsservice.ConversationsService
	HabitsRpc          clienthabits.Habits
	GoalsRpc           clientgoals.Goals
	CategoriesRpc      clientcategories.Categories
	ArticlesRpc        clientarticles.Articles
	CheckInRpc         checkinservice.CheckInService
	ActivityRpc        clientactivity.Activity
	WeeklyReviewRpc    clientweeklyreview.WeeklyReviewService
	PersonalizationRpc clientpersonalization.PersonalizationService
	BillingRpc         clientbilling.BillingService
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

	// Create client with auth propagation interceptor
	clientOpts := []zrpc.ClientOption{
		zrpc.WithDialOption(grpc.WithUnaryInterceptor(mdpropagate.UnaryClientInterceptor())),
	}

	authRpc := authservice.NewAuthService(zrpc.MustNewClient(c.AuthRpc, clientOpts...))
	var stripeClient *stripe.Client
	if c.Billing.StripeSecretKey != "" {
		stripeClient = stripe.NewClient(c.Billing.StripeSecretKey)
	}

	return &ServiceContext{
		Config: c,
		Auth: middleware.JWTMiddleware(middleware.JWTVerifierConfig{
			Secret:   c.Auth.Secret,
			Issuer:   c.Auth.Issuer,
			Audience: c.Auth.Audience,
		}),
		AuthRpc:            authRpc,
		NotificationsRpc:   notificationsClient.NewNotifications(zrpc.MustNewClient(c.NotificationsRpc, clientOpts...)),
		SavedRpc:           clientsaved.NewSaved(zrpc.MustNewClient(c.ClientRpc, clientOpts...)),
		SettingsRpc:        clientsettings.NewSettings(zrpc.MustNewClient(c.ClientRpc, clientOpts...)),
		ReportRpc:          clientreport.NewReport(zrpc.MustNewClient(c.ClientRpc, clientOpts...)),
		SearchRpc:          searchservice.NewSearchService(zrpc.MustNewClient(c.SearchRpc, clientOpts...)),
		ConversationsRpc:   conversationsservice.NewConversationsService(zrpc.MustNewClient(c.ConversationsRpc, clientOpts...)),
		HabitsRpc:          clienthabits.NewHabits(zrpc.MustNewClient(c.ClientRpc, clientOpts...)),
		GoalsRpc:           clientgoals.NewGoals(zrpc.MustNewClient(c.ClientRpc, clientOpts...)),
		CategoriesRpc:      clientcategories.NewCategories(zrpc.MustNewClient(c.ClientRpc, clientOpts...)),
		ArticlesRpc:        clientarticles.NewArticles(zrpc.MustNewClient(c.ClientRpc, clientOpts...)),
		CheckInRpc:         checkinservice.NewCheckInService(zrpc.MustNewClient(c.ClientRpc, clientOpts...)),
		ActivityRpc:        clientactivity.NewActivity(zrpc.MustNewClient(c.ClientRpc, clientOpts...)),
		WeeklyReviewRpc:    clientweeklyreview.NewWeeklyReviewService(zrpc.MustNewClient(c.ClientRpc, clientOpts...)),
		PersonalizationRpc: clientpersonalization.NewPersonalizationService(zrpc.MustNewClient(c.ClientRpc, clientOpts...)),
		BillingRpc:         clientbilling.NewBillingService(zrpc.MustNewClient(c.ClientRpc, clientOpts...)),
		StripeClient:       stripeClient,
	}
}
