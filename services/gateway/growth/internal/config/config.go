// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package config

import (
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	rest.RestConf
	AuthRpc          zrpc.RpcClientConf
	NotificationsRpc zrpc.RpcClientConf
	ClientRpc        zrpc.RpcClientConf
	SearchRpc        zrpc.RpcClientConf
	ConversationsRpc zrpc.RpcClientConf
	AICoachRpc       zrpc.RpcClientConf
	Auth             struct {
		Secret   string
		Issuer   string
		Audience string
	}
	Billing struct {
		Mode                string // disabled, fake_door, stripe_test, stripe_live
		StripeSecretKey     string
		StripeWebhookSecret string
		FrontendURL         string
	}
	ServiceAuth struct {
		Secret string
	}
}
