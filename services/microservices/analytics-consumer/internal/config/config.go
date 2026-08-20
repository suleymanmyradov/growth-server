package config

import (
	"time"

	"github.com/zeromicro/go-zero/core/trace"
)

type Config struct {
	Postgres struct {
		Datasource      string `json:",optional" secret:"true"`
		MaxOpenConns    int
		MaxIdleConns    int
		ConnMaxLifetime time.Duration
	}
	Kafka struct {
		Brokers       []string
		EventsTopic   string
		ConsumerGroup string `json:",optional"`
		DLQTopic      string `json:",optional"`
		Processors    int    `json:",optional"`
		Consumers     int    `json:",optional"`
	}
	Telemetry trace.Config `json:",optional"`
}
