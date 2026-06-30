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
	}
}
