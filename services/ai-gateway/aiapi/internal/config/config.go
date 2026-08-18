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
	// Coaching holds tunable limits for the agentic coaching stream.
	// All fields are optional; defaults are applied in the handler.
	Coaching CoachingConfig `json:",optional"`
}

// CoachingConfig holds tunable limits for the agentic coaching stream.
// These are call-level parameters (not LLM client config) and are specific
// to the ai-gateway's coaching endpoint.
type CoachingConfig struct {
	// MaxSteps limits the model<->tool round-trip loop per coaching turn.
	// Defaults to 6 when zero.
	MaxSteps int `json:",optional"`
	// MaxTotalTokens limits the cumulative prompt + completion tokens across
	// all agent steps in a coaching turn. Defaults to 100000 when zero.
	// Raise this if the agent is being cut off mid-response; lower it to
	// cap per-turn spend.
	MaxTotalTokens int `json:",optional"`
}
