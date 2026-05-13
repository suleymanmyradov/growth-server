package config

import (
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
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
	AI ai.Config
}
