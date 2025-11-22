package svc

import "github.com/suleymanmyradov/growth-server/services/gateway/services/activity/rpc/internal/config"

type ServiceContext struct {
	Config config.Config
}

func NewServiceContext(c config.Config) *ServiceContext {
	return &ServiceContext{
		Config: c,
	}
}
