package config

import (
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type AuthConfig struct {
	Secret   string
	Issuer   string
	Audience string
}

type Config struct {
	rest.RestConf
	CORS struct {
		Origins []string
	}
	Auth             AuthConfig
	AuthRpc          zrpc.RpcClientConf
	ClientRpc        zrpc.RpcClientConf
	SearchRpc        zrpc.RpcClientConf
	ConversationsRpc zrpc.RpcClientConf
}
