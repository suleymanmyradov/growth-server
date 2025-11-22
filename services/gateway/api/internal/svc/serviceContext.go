// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"github.com/suleymanmyradov/growth-server/services/gateway/api/internal/config"
	"github.com/suleymanmyradov/growth-server/services/gateway/api/internal/middleware"
	authclient "github.com/suleymanmyradov/growth-server/services/gateway/services/auth/rpc/authClient"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/habits/rpc/habitsservice"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config                 config.Config
	AuthRpc                authclient.Auth
	HabitsRpc              habitsservice.HabitsService
	RequiredAuthMiddleware rest.Middleware
	OptionalAuthMiddleware rest.Middleware
}

func NewServiceContext(c config.Config) *ServiceContext {
	authCli := authclient.NewAuth(zrpc.MustNewClient(c.AuthRpc))
	habitsCli := habitsservice.NewHabitsService(zrpc.MustNewClient(c.HabitsRpc))

	return &ServiceContext{
		Config:                 c,
		AuthRpc:                authCli,
		HabitsRpc:              habitsCli,
		RequiredAuthMiddleware: middleware.NewRequiredAuthMiddleware(authCli).Handle,
		OptionalAuthMiddleware: middleware.NewOptionalAuthMiddleware(authCli).Handle,
	}
}
