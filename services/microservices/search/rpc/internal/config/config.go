package config

import (
	"github.com/suleymanmyradov/growth-server/pkg/auth/jwt"
	"github.com/suleymanmyradov/growth-server/pkg/auth/s2s"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	Meili struct {
		Host   string
		APIKey string `json:",optional" secret:"true"`
		Index  string
	}
	JWT         jwt.Config `json:",optional"`
	ServiceAuth s2s.Config `json:",optional"`
}
