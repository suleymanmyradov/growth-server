// Package main provides the search RPC service entry point.
package main

import (
	"flag"
	"fmt"

	"github.com/suleymanmyradov/growth-server/pkg/auth/jwt"
	"github.com/suleymanmyradov/growth-server/pkg/auth/mdpropagate"
	"github.com/suleymanmyradov/growth-server/pkg/auth/s2s"
	"github.com/suleymanmyradov/growth-server/pkg/configsafe"
	"github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/internal/server"
	"github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/pb/search"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
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
	logx.Infof("starting search service with config: %+v", configsafe.MaskSecrets(c))
	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		search.RegisterSearchServiceServer(grpcServer, server.NewSearchServiceServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()

	// Verify JWT tokens so downstream services don't blindly trust gateway headers.
	// In production, replace this with jwt.NewVerifier using a public key (RS256/ES256).
	tokenVerifier, err := jwt.NewTokenMaker(jwt.Config{
		Secret:   c.Auth.Secret,
		Issuer:   c.Auth.Issuer,
		Audience: c.Auth.Audience,
	}, nil)
	if err != nil {
		logx.Must(err)
	}

	s.AddUnaryInterceptors(
		mdpropagate.UnaryServerInterceptorOptional(tokenVerifier),
		s2s.UnaryServerInterceptor(s2s.Config{Secret: c.ServiceAuth.Secret}),
	)

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
