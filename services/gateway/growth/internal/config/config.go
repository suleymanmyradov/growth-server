// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package config

import (
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/middleware"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	rest.RestConf
	AuthRpc          zrpc.RpcClientConf
	NotificationsRpc zrpc.RpcClientConf
	ClientRpc        zrpc.RpcClientConf
	SearchRpc        zrpc.RpcClientConf
	FileManagerRpc   zrpc.RpcClientConf
	Auth             struct {
		// PublicKey is the ES256 public key (PEM) for verifying tokens minted
		// by the auth service. Secret is the legacy HS256 fallback kept only
		// for the dual-verify migration window.
		PublicKey string `json:",optional"`
		Secret    string `json:",optional" secret:"true"`
		Issuer    string `json:",optional"`
		Audience  string `json:",optional"`
	}
	ServiceAuth struct {
		Secret string `json:",optional" secret:"true"`
	}
	RateLimit middleware.RateLimitConfig
}
