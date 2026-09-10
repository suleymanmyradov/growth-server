package main

import (
	"flag"
	"fmt"

	"github.com/suleymanmyradov/growth-server/pkg/auth/s2s"
	"github.com/suleymanmyradov/growth-server/pkg/server/recovery"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/internal/server"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/pb/filemanager"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/filemanager.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c, conf.UseEnv())
	logx.Must(c.ServiceAuth.MustValidate())
	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		filemanager.RegisterFileManagerServer(grpcServer, server.NewFileManagerServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()

	// Every caller (gateway, adminway) must present a valid s2s HMAC signature.
	s.AddUnaryInterceptors(
		recovery.UnaryServerInterceptor(),
		s2s.UnaryServerInterceptor(c.ServiceAuth),
	)

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
