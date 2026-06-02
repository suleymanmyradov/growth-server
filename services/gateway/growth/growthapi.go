// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package main

import (
	"flag"
	"fmt"

	"github.com/suleymanmyradov/growth-server/pkg/configsafe"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/config"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/handler"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/middleware"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/growthapi.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	logx.Infof("starting gateway with config: %+v", configsafe.MaskSecrets(c))

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	server.Use(middleware.ResponseShapeMiddleware())

	ctx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, ctx)

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
