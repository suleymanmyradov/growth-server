package config

import (
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/auth/jwt"
	"github.com/suleymanmyradov/growth-server/pkg/auth/s2s"
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
	Kafka struct {
		Brokers           []string
		EventsTopic       string
		ReminderDueTopic  string
		ConsumerGroup     string
	}
	JWT         jwt.Config `json:",optional"`
	ServiceAuth s2s.Config `json:",optional"`
}
