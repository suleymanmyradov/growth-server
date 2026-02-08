package main

import (
	"flag"
	"fmt"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/config"
	activityServer "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/server/activity"
	articlesServer "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/server/articles"
	notificationsServer "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/server/notifications"
	reportServer "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/server/report"
	savedServer "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/server/saved"
	settingsServer "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/server/settings"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/client.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		client.RegisterActivityServer(grpcServer, activityServer.NewActivityServer(ctx))
		client.RegisterNotificationsServer(grpcServer, notificationsServer.NewNotificationsServer(ctx))
		client.RegisterReportServer(grpcServer, reportServer.NewReportServer(ctx))
		client.RegisterSavedServer(grpcServer, savedServer.NewSavedServer(ctx))
		client.RegisterSettingsServer(grpcServer, settingsServer.NewSettingsServer(ctx))
		client.RegisterArticlesServer(grpcServer, articlesServer.NewArticlesServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
