package main

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/suleymanmyradov/growth-server/pkg/configsafe"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/config"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/handler"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/handler/files"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/adminapi.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	logx.Infof("starting adminway with config: %+v", configsafe.MaskSecrets(c))

	if c.MaxConns == 0 {
		c.MaxConns = 10000
	}
	if c.MaxBytes == 0 {
		c.MaxBytes = 1 << 20
	}

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	ctx := svc.NewServiceContext(c)
	defer ctx.Close()

	handler.RegisterHandlers(server, ctx)

	server.AddRoutes(
		rest.WithMiddlewares(
			[]rest.Middleware{ctx.Auth, ctx.AdminAuth},
			[]rest.Route{
				{
					Method:  http.MethodPost,
					Path:    "/admin/articles",
					Handler: files.AdminUploadArticleImageHandler(ctx),
				},
			}...,
		),
		rest.WithPrefix("/api/v1"),
	)

	fmt.Printf("Starting admin server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
