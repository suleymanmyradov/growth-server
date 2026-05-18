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
	Kafka struct {
		Brokers     []string
		EventsTopic string
	}
}
