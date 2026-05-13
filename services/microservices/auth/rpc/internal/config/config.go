package config

import (
	"time"

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
	Cache struct {
		Redis struct {
			Addr     string
			Password string
			DB       int
			Prefix   string
		}
	}
	JWT struct {
		Secret                string
		Issuer                string
		Audience              string
		AccessExpiryDuration  time.Duration
		RefreshExpiryDuration time.Duration
	}
}
