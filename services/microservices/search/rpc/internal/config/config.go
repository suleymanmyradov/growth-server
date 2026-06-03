package config

import "github.com/zeromicro/go-zero/zrpc"

type Config struct {
	zrpc.RpcServerConf
	Meili struct {
		Host   string
		APIKey string `secret:"true"`
		Index  string
	}
	Auth struct {
		Secret   string `secret:"true"`
		Issuer   string
		Audience string
	}
	ServiceAuth struct {
		Secret string `secret:"true"`
	}
}
