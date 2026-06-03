package config

import (
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
)

type Config struct {
	Postgres struct {
		Datasource      string `secret:"true"`
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
}
