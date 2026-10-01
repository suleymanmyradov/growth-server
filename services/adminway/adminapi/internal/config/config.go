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
	// Mfa configures TOTP multi-factor auth for admin accounts.
	Mfa struct {
		// Issuer is the TOTP issuer label shown in authenticator apps;
		// "Growth Admin" when unset.
		Issuer string `json:",optional"`
		// EncryptionKey is a base64-encoded 32-byte AES-256 key protecting TOTP
		// secrets at rest. Generate with `openssl rand -base64 32`.
		EncryptionKey string `json:",optional" secret:"true"`
		// Required forces every admin through enrollment at login (an
		// enroll-purpose ticket replaces the token pair until TOTP is set up).
		Required bool `json:",optional"`
		// TicketTTL bounds how long a pre-auth mfa ticket stays valid.
		TicketTTL time.Duration `json:",default=5m"`
	} `json:",optional"`
	// RateLimit holds the Redis-backed per-IP quotas for the unauthenticated
	// auth routes (login/refresh brute-force protection).
	RateLimit   middleware.RateLimitConfig
	ServiceAuth struct {
		Secret string `json:",optional" secret:"true"`
	}
}
