package main

import (
	"context"
	"flag"

	"github.com/suleymanmyradov/growth-server/pkg/auth/mdpropagate"
	"github.com/suleymanmyradov/growth-server/pkg/server/runtime"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/config"
	activityServer "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/server/activity"
	articlesServer "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/server/articles"
	billingServiceServer "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/server/billingservice"
	categoriesServer "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/server/categories"
	checkInServiceServer "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/server/checkinservice"
	goalsServer "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/server/goals"
	habitsServer "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/server/habits"
	personalizationServiceServer "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/server/personalizationservice"
	reportServer "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/server/report"
	savedServer "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/server/saved"
	settingsServer "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/server/settings"
	weeklyReviewServer "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/server/weeklyreviewservice"
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
		client.RegisterReportServer(grpcServer, reportServer.NewReportServer(ctx))
		client.RegisterSavedServer(grpcServer, savedServer.NewSavedServer(ctx))
		client.RegisterSettingsServer(grpcServer, settingsServer.NewSettingsServer(ctx))
		client.RegisterArticlesServer(grpcServer, articlesServer.NewArticlesServer(ctx))
		client.RegisterGoalsServer(grpcServer, goalsServer.NewGoalsServer(ctx))
		client.RegisterHabitsServer(grpcServer, habitsServer.NewHabitsServer(ctx))
		client.RegisterCategoriesServer(grpcServer, categoriesServer.NewCategoriesServer(ctx))
		client.RegisterCheckInServiceServer(grpcServer, checkInServiceServer.NewCheckInServiceServer(ctx))
		client.RegisterWeeklyReviewServiceServer(grpcServer, weeklyReviewServer.NewWeeklyReviewServiceServer(ctx))
		client.RegisterPersonalizationServiceServer(grpcServer, personalizationServiceServer.NewPersonalizationServiceServer(ctx))
		client.RegisterBillingServiceServer(grpcServer, billingServiceServer.NewBillingServiceServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})

	s.AddUnaryInterceptors(mdpropagate.UnaryServerInterceptorOptional())

	runtime.Run(s.Start, runtime.Options{
		RPC: s,
		OnShutdown: []func(context.Context) error{
			func(_ context.Context) error {
				ctx.Close()
				return nil
			},
		},
	})
}
