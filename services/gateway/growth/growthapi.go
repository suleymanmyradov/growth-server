// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package main

import (
	"flag"
	"fmt"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/config"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/handler"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/prometheus"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/growthapi.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	corsOrigins := c.CORS.Origins
	if len(corsOrigins) == 0 {
		corsOrigins = []string{"http://localhost:3000"}
	}

	server := rest.MustNewServer(c.RestConf, rest.WithCors(corsOrigins...))
	defer server.Stop()

	if c.Prometheus.Host != "" {
		prometheus.StartAgent(c.Prometheus)
	}

	ctx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, ctx)

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
