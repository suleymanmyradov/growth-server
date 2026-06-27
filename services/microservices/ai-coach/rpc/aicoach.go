package main

import (
	"flag"
	"fmt"

	"github.com/suleymanmyradov/growth-server/pkg/server/recovery"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/config"
	aicoachserver "github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/server/aicoachservice"
	conversationserver "github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/server/conversationservice"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/pb/aicoach"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/aicoach.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		aicoach.RegisterAICoachServiceServer(grpcServer, aicoachserver.NewAICoachServiceServer(ctx))
		aicoach.RegisterConversationServiceServer(grpcServer, conversationserver.NewConversationServiceServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()

	s.AddUnaryInterceptors(recovery.UnaryServerInterceptor())

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
