// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package svc

import (
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/config"
	authservice "github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/authservice"
	clientnotifications "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/notifications"
	conversationsservice "github.com/suleymanmyradov/growth-server/services/microservices/conversations/rpc/client/conversationsservice"
	searchservice "github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/searchservice"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config           config.Config
	AuthRpc          authservice.AuthService
	ClientRpc        clientnotifications.Notifications
	SearchRpc        searchservice.SearchService
	ConversationsRpc conversationsservice.ConversationsService
}

func NewServiceContext(c config.Config) *ServiceContext {
	return &ServiceContext{
		Config:           c,
		AuthRpc:          authservice.NewAuthService(zrpc.MustNewClient(c.AuthRpc)),
		ClientRpc:        clientnotifications.NewNotifications(zrpc.MustNewClient(c.ClientRpc)),
		SearchRpc:        searchservice.NewSearchService(zrpc.MustNewClient(c.SearchRpc)),
		ConversationsRpc: conversationsservice.NewConversationsService(zrpc.MustNewClient(c.ConversationsRpc)),
	}
}
