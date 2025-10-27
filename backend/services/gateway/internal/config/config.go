// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/rest"
)

type Config struct {
	rest.RestConf
	DataSource string
	Redis      cache.CacheConf
	Auth       AuthConfig
	MeiliSearch MeiliSearchConfig
}

type AuthConfig struct {
	AccessSecret  string
	AccessExpire  int64
	RefreshSecret string
	RefreshExpire int64
}

type MeiliSearchConfig struct {
	Host     string
	MasterKey string
}
