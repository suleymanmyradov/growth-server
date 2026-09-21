package config

import (
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
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
		ConsumerGroup string
		DLQTopic      string
		ForceCommit   bool
		Processors    int
		Consumers     int
	}
	// Redis is used for Redis Streams event transport when Kafka brokers
	// are not configured.
	Redis struct {
		Addr     string `json:",optional"`
		Password string `json:",optional" secret:"true"`
		DB       int    `json:",optional"`
	}
	Consumer struct {
		Timeout     time.Duration
		Concurrency int
	}
	AI ai.Config
	// Telemetry configures distributed tracing for the consumer.
	Telemetry trace.Config `json:",optional"`
	// Prometheus serves /metrics when Host is set (scraped in prod).
	Prometheus prometheus.Config `json:",optional"`
	// Log configures logx output (JSON encoding in prod).
	Log logx.LogConf `json:",optional"`
}
