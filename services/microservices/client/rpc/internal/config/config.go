package config

import (
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/auth/jwt"
	"github.com/suleymanmyradov/growth-server/pkg/auth/s2s"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	Postgres struct {
		Datasource      string `json:",optional" secret:"true"`
		MaxOpenConns    int
		MaxIdleConns    int
		ConnMaxLifetime time.Duration
	}
	Kafka struct {
		Brokers       []string
		EventsTopic   string
		ConsumerGroup string `json:",optional"`
	}
	Billing struct {
		Mode                string `json:",optional"`
		StripeSecretKey     string `json:",optional" secret:"true"`
		StripeWebhookSecret string `json:",optional" secret:"true"`
		FrontendURL         string `json:",optional"`
		// RevenueCat configuration for mobile subscriptions (App Store / Play
		// Store). See pkg/revenuecat/client.go and docs/push-notifications-design.md.
		RevenueCat struct {
			// APIKey is the RevenueCat secret API key (starts with "sk_").
			// Required for REST API calls (entitlement fetch, backfill).
			APIKey string `json:",optional" secret:"true"`
			// ProjectID is the RevenueCat project ID.
			ProjectID string `json:",optional"`
			// WebhookSecret is the bearer token configured in the RevenueCat
			// dashboard for webhook authentication.
			WebhookSecret string `json:",optional" secret:"true"`
			// Enabled toggles RevenueCat webhook processing. When false, the
			// webhook handler rejects all requests.
			Enabled bool `json:",optional,default=false"`
		} `json:",optional"`
	}
	JWT         jwt.Config `json:",optional"`
	ServiceAuth s2s.Config `json:",optional"`
	AppRedis    struct {
		Addr     string
		Password string `json:",optional" secret:"true"`
		DB       int
	}
	WeeklyReview struct {
		RegenerationCooldown time.Duration `json:",optional"`
	}
}
