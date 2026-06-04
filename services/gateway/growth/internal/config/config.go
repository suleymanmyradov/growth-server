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
	AICoachRpc       zrpc.RpcClientConf
	Auth             struct {
		Secret   string `json:",optional" secret:"true"`
		Issuer   string `json:",optional"`
		Audience string `json:",optional"`
	}
	Billing struct {
		Mode                string `json:",optional"`
		StripeSecretKey     string `json:",optional" secret:"true"`
		StripeWebhookSecret string `json:",optional" secret:"true"`
		FrontendURL         string `json:",optional"`
	}
	ServiceAuth struct {
		Secret string `json:",optional" secret:"true"`
	}
	RateLimit middleware.RateLimitConfig
}
