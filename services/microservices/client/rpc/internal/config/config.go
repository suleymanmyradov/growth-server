package config

import (
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	Postgres struct {
		Datasource      string
		MaxOpenConns    int
		MaxIdleConns    int
		ConnMaxLifetime time.Duration
	}
	Kafka struct {
		Brokers     []string
		EventsTopic string
	}
	AI      ai.Config
	Billing struct {
		Mode            string // disabled, fake_door, stripe_test, stripe_live
		StripeSecretKey string
		StripeWebhookSecret string
		FrontendURL     string
	}
}
