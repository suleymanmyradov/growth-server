package main

import (
	"flag"
	"fmt"

	"github.com/suleymanmyradov/growth-server/pkg/auth/mdpropagate"
	"github.com/suleymanmyradov/growth-server/pkg/auth/s2s"
	"github.com/suleymanmyradov/growth-server/pkg/configsafe"
	"github.com/suleymanmyradov/growth-server/services/microservices/conversations/rpc/internal/config"
	conversationsserviceServer "github.com/suleymanmyradov/growth-server/services/microservices/conversations/rpc/internal/server/conversationsservice"
	"github.com/suleymanmyradov/growth-server/services/microservices/conversations/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/conversations/rpc/pb/conversations"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/conversations.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	logx.Infof("starting conversations service with config: %+v", configsafe.MaskSecrets(c))
	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		conversations.RegisterConversationsServiceServer(grpcServer, conversationsserviceServer.NewConversationsServiceServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()

	s.AddUnaryInterceptors(
		mdpropagate.UnaryServerInterceptor(),
		s2s.UnaryServerInterceptor(s2s.Config{Secret: c.ServiceAuth.Secret}),
	)

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
