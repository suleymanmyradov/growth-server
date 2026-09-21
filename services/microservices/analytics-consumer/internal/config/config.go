package config

import (
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/prometheus"
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
	// Redis is used for Redis Streams event transport when Kafka brokers
	// are not configured.
	Redis struct {
		Addr     string `json:",optional"`
		Password string `json:",optional" secret:"true"`
		DB       int    `json:",optional"`
	}
	Telemetry trace.Config `json:",optional"`
	// Prometheus serves /metrics when Host is set (scraped in prod).
	Prometheus prometheus.Config `json:",optional"`
	// Log configures logx output (JSON encoding in prod).
	Log logx.LogConf `json:",optional"`
}
