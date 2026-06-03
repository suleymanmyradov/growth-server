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
	Kafka struct {
		Brokers          []string
		EventsTopic      string
		ReminderDueTopic string
		ConsumerGroup    string
	}
	Auth struct {
		Secret   string `secret:"true"`
		Issuer   string
		Audience string
	}
	ServiceAuth struct {
		Secret string `secret:"true"`
	}
}
