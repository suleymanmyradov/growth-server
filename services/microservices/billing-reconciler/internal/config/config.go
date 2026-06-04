package config

import (
	"time"
)

type Config struct {
	Postgres struct {
		Datasource      string `secret:"true"`
		MaxOpenConns    int
		MaxIdleConns    int
		ConnMaxLifetime time.Duration
	}
}
