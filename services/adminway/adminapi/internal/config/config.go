package config

import (
	"time"

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
	Auth           struct {
		Secret                string        `json:",optional" secret:"true"`
		Issuer                string        `json:",optional"`
		Audience              string        `json:",optional"`
		AccessExpiryDuration  time.Duration `json:",optional"`
		RefreshExpiryDuration time.Duration `json:",optional"`
	}
	ServiceAuth struct {
		Secret string `json:",optional" secret:"true"`
	}
}
