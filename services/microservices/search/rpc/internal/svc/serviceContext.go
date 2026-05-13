// Package svc provides service context for the search RPC service.
package svc

import "github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/internal/config"

// ServiceContext holds the service configuration.
type ServiceContext struct {
	Config config.Config
}

// NewServiceContext creates a new ServiceContext.
func NewServiceContext(c config.Config) *ServiceContext {
	return &ServiceContext{
		Config: c,
	}
}
