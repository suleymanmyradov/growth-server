package config

import (
	"time"

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
	Cache struct {
		Redis struct {
			Addr     string
			Password string `json:",optional" secret:"true"`
			DB       int
			Prefix   string
		}
	}
	JWT struct {
		Secret                string        `json:",optional" secret:"true"`
		Issuer                string        `json:",optional"`
		Audience              string        `json:",optional"`
		AccessExpiryDuration  time.Duration `json:",optional"`
		RefreshExpiryDuration time.Duration `json:",optional"`
	}
	Email struct {
		// Provider is "resend" by default. An empty APIKey enables a noop sender
		// so local development works without email credentials.
		Provider    string `json:",optional"`
		APIKey      string `json:",optional" secret:"true"`
		FromAddress string `json:",optional"`
		// FrontendBaseURL is the public origin used to build action links, e.g.
		// https://app.example.com (no trailing slash).
		FrontendBaseURL string `json:",optional"`
	}
	GoogleOAuth struct {
		ClientID     string `json:",optional" secret:"true"`
		ClientSecret string `json:",optional" secret:"true"`
		// RedirectURI registered in Google Cloud Console for this client.
		RedirectURI string `json:",optional"`
		// AllowedRedirectURIs is an explicit allowlist of redirect URIs that
		// clients may supply in the GoogleLogin request. If empty, only the
		// configured RedirectURI is accepted. This prevents authorization code
		// interception via arbitrary redirect URIs.
		AllowedRedirectURIs []string `json:",optional"`
	}
	AppleOAuth struct {
		// ServiceID is the Apple Services ID used for Sign in with Apple (the
		// audience of the ID token). Required for ID token verification.
		ServiceID string `json:",optional"`
		// TeamID is the Apple Developer Team ID. Required for code exchange.
		TeamID string `json:",optional"`
		// KeyID is the ID of the private key (.p8) used to sign the client
		// secret JWT for code exchange. Required for code exchange.
		KeyID string `json:",optional"`
		// PrivateKeyPath is the filesystem path to the .p8 private key (ECDSA
		// P-256). Mutually exclusive with PrivateKey. Required for code exchange.
		PrivateKeyPath string `json:",optional" secret:"true"`
		// PrivateKey is the inline PEM-encoded .p8 private key. Mutually
		// exclusive with PrivateKeyPath. Required for code exchange.
		PrivateKey string `json:",optional" secret:"true"`
		// RedirectURI registered with Apple for code exchange. Not required for
		// ID token verification.
		RedirectURI string `json:",optional"`
		// AllowedRedirectURIs is an explicit allowlist of redirect URIs that
		// clients may supply when exchanging an authorization code. If empty,
		// only the configured RedirectURI is accepted. Mirrors the Google flow.
		AllowedRedirectURIs []string `json:",optional"`
	}
	Kafka struct {
		Brokers     []string `json:",optional"`
		EventsTopic string   `json:",optional"`
	}
}
