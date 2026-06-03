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
	Cache struct {
		Redis struct {
			Addr     string
			Password string `secret:"true"`
			DB       int
			Prefix   string
		}
	}
	JWT struct {
		Secret                string `secret:"true"`
		Issuer                string
		Audience              string
		AccessExpiryDuration  time.Duration
		RefreshExpiryDuration time.Duration
	}
}
