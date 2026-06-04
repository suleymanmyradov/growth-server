package config

import (
	"time"

	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	Postgres struct {
		Datasource      string        `json:",optional" secret:"true"`
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
}
