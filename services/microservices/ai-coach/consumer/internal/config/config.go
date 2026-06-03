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
	}
	AI ai.Config
}
