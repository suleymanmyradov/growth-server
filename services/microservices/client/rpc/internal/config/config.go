package config

import (
	"time"

	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	Postgres struct {
		Datasource      string `secret:"true"`
		MaxOpenConns    int
		MaxIdleConns    int
		ConnMaxLifetime time.Duration
	}
	Kafka struct {
		Brokers     []string
		EventsTopic string
	}
	AICoachRpc zrpc.RpcClientConf
	Billing    struct {
		Mode                string // disabled, fake_door, stripe_test, stripe_live
		StripeSecretKey     string `secret:"true"`
		StripeWebhookSecret string `secret:"true"`
		FrontendURL         string
	}
	Auth struct {
		Secret   string `secret:"true"`
		Issuer   string
		Audience string
	}
	ServiceAuth struct {
		Secret string `secret:"true"`
	}
	Redis struct {
		Addr     string
		Password string `secret:"true"`
		DB       int
	}
}
