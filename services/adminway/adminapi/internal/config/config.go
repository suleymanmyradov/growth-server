package config

import (
	"time"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/middleware"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	rest.RestConf
	Postgres struct {
		Datasource      string        `json:",optional" secret:"true"`
		MaxOpenConns    int           `json:",default=25"`
		MaxIdleConns    int           `json:",default=5"`
		ConnMaxLifetime time.Duration `json:",default=5m"`
	}
	ClientRpc      zrpc.RpcClientConf
	FileManagerRpc zrpc.RpcClientConf
	SearchRpc      zrpc.RpcClientConf
	AuthRpc        zrpc.RpcClientConf `json:",optional"`
	Kafka          struct {
		Brokers     []string `json:",optional"`
		EventsTopic string   `json:",optional"`
	} `json:",optional"`
	// Redis is used for Redis Streams event transport when Kafka brokers
	// are not configured.
	Redis struct {
		Addr     string `json:",optional"`
		Password string `json:",optional" secret:"true"`
		DB       int    `json:",optional"`
	} `json:",optional"`
	Auth struct {
		// PrivateKey is adminway's own PEM-encoded ES256 signing key (the
		// growth-admin audience has a separate keypair from user tokens).
		PrivateKey            string        `json:",optional" secret:"true"`
		PublicKey             string        `json:",optional"`
		Issuer                string        `json:",optional"`
		Audience              string        `json:",optional"`
		AccessExpiryDuration  time.Duration `json:",optional"`
		RefreshExpiryDuration time.Duration `json:",optional"`
	}
	// RateLimit holds the Redis-backed per-IP quotas for the unauthenticated
	// auth routes (login/refresh brute-force protection).
	RateLimit   middleware.RateLimitConfig
	ServiceAuth struct {
		Secret string `json:",optional" secret:"true"`
	}
}
