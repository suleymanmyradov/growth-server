package svc

import "github.com/suleymanmyradov/growth-server/services/gateway/services/saved/rpc/internal/config"

type ServiceContext struct {
	Config config.Config
}

func NewServiceContext(c config.Config) *ServiceContext {
	return &ServiceContext{
		Config: c,
	}
}
