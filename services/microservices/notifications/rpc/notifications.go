package main

import (
	"context"
	"flag"

	"github.com/suleymanmyradov/growth-server/pkg/auth/jwt"
	"github.com/suleymanmyradov/growth-server/pkg/auth/mdpropagate"
	"github.com/suleymanmyradov/growth-server/pkg/auth/s2s"
	"github.com/suleymanmyradov/growth-server/pkg/configsafe"
	"github.com/suleymanmyradov/growth-server/pkg/server/recovery"
	"github.com/suleymanmyradov/growth-server/pkg/server/runtime"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/server"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/pb/notifications"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
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
	logx.Infof("starting notifications service with config: %+v", configsafe.MaskSecrets(c))
	ctx := svc.NewServiceContext(c)

	// Start Kafka consumers and scheduler before RPC server.
	cancelConsumers := ctx.StartConsumers()

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		notifications.RegisterNotificationsServer(grpcServer, server.NewNotificationsServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})

	// Verify JWT tokens so downstream services don't blindly trust gateway headers.
	// In production, replace this with jwt.NewVerifier using a public key (RS256/ES256).
	tokenVerifier, err := jwt.NewTokenMaker(c.JWT, nil)
	if err != nil {
		logx.Must(err)
	}

	s.AddUnaryInterceptors(
		recovery.UnaryServerInterceptor(),
		mdpropagate.UnaryServerInterceptor(tokenVerifier),
		s2s.UnaryServerInterceptor(c.ServiceAuth),
	)

	runtime.Run(func(_ context.Context) { s.Start() }, runtime.Options{
		RPC: s,
		OnShutdown: []func(context.Context) error{
			func(_ context.Context) error {
				cancelConsumers()
				ctx.Close()
				return nil
			},
		},
	})
}
