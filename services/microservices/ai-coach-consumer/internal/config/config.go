package config

import (
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/zeromicro/go-zero/core/trace"
)

type Config struct {
	Postgres struct {
		Datasource      string        `json:",optional" secret:"true"`
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
	Consumer struct {
		Timeout     time.Duration
		Concurrency int
	}
	AI ai.Config
	// Telemetry configures distributed tracing for the consumer.
	Telemetry trace.Config `json:",optional"`
}
