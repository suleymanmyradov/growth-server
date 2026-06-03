package config

import (
	"time"

	"github.com/zeromicro/go-zero/core/service"
)

type Config struct {
	service.ServiceConf
	Postgres struct {
		Datasource      string `secret:"true"`
		MaxOpenConns    int
		MaxIdleConns    int
		ConnMaxLifetime time.Duration
	}
	Meili struct {
		Host   string
		APIKey string `secret:"true"`
		Index  string
	}
	Sync struct {
		PollInterval time.Duration
		BatchSize    int
		LockTimeout  time.Duration
		MaxAttempts  int
		WorkerID     string
	}
	Backfill bool
}
