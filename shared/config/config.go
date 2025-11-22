package config

import (
	"github.com/zeromicro/go-zero/rest"

	"github.com/suleymanmyradov/growth-server/third_party/cache"
	"github.com/suleymanmyradov/growth-server/third_party/database"
	"github.com/suleymanmyradov/growth-server/third_party/search"
)

type Config struct {
	rest.RestConf
	Database    database.PostgresConfig
	Redis       cache.RedisConfig
	MeiliSearch search.MeiliSearchConfig
	Auth        AuthConfig
}

type AuthConfig struct {
	AccessSecret  string `json:",env=AUTH_ACCESS_SECRET"`
	AccessExpire  int64  `json:",env=AUTH_ACCESS_EXPIRE"`
	RefreshSecret string `json:",env=AUTH_REFRESH_SECRET"`
	RefreshExpire int64  `json:",env=AUTH_REFRESH_EXPIRE"`
}

type ServiceConfig struct {
	Name        string `json:",env"`
	Host        string `json:",env"`
	Port        int    `json:",env"`
	Database    database.PostgresConfig
	Redis       cache.RedisConfig
	MeiliSearch search.MeiliSearchConfig
	Auth        AuthConfig
}
