package main

import (
	"flag"
	"fmt"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/search/rpc/internal/config"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/search/rpc/internal/server"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/search/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/search/rpc/search"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/search.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		search.RegisterSearchServiceServer(grpcServer, server.NewSearchServiceServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
