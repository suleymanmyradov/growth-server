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
		Brokers     []string
		EventsTopic string
	}
	AICoachRpc     zrpc.RpcClientConf
	FileManagerRpc zrpc.RpcClientConf
	Billing        struct {
		Mode                string `json:",optional"`
		StripeSecretKey     string `json:",optional" secret:"true"`
		StripeWebhookSecret string `json:",optional" secret:"true"`
		FrontendURL         string `json:",optional"`
	}
	JWT         jwt.Config `json:",optional"`
	ServiceAuth s2s.Config `json:",optional"`
	AppRedis    struct {
		Addr     string
		Password string `json:",optional" secret:"true"`
		DB       int
	}
}
