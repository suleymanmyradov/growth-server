// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package config

import (
	"github.com/suleymanmyradov/growth-server/pkg/ai"
	sharedmw "github.com/suleymanmyradov/growth-server/pkg/httpx/middleware"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	rest.RestConf
	AuthRpc    zrpc.RpcClientConf
	ClientRpc  zrpc.RpcClientConf
	AICoachRpc zrpc.RpcClientConf
	SearchRpc  zrpc.RpcClientConf `json:",optional"`
	Auth       struct {
		Secret   string `json:",optional" secret:"true"`
		Issuer   string `json:",optional"`
		Audience string `json:",optional"`
	}
	ServiceAuth struct {
		Secret string `json:",optional" secret:"true"`
	}
	RateLimit sharedmw.RateLimitConfig
	// AI configures the LLM client used by the agentic coaching flow
	// (StreamAgent with on-demand tool calls). Required — the agentic path
	// is the only coaching path served by this gateway.
	AI ai.Config
}
