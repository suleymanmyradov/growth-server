package main

import (
	"flag"
	"fmt"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/notifications/rpc/internal/config"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/notifications/rpc/internal/server"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/notifications/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/notifications/rpc/notifications"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/notifications.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		notifications.RegisterNotificationsServiceServer(grpcServer, server.NewNotificationsServiceServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
