// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/suleymanmyradov/growth-server/pkg/httpx/errors"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/config"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/handler"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

var configFile = flag.String("f", "etc/adminapi.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c, conf.UseEnv())

	// Install a custom error handler so gRPC status errors returned from logic
	// layers are mapped to proper HTTP status codes with JSON bodies, instead
	// of go-zero's default (plain-text 400 for non-gRPC errors, or plain-text
	// gRPC code mapping). Logic that returns raw (non-status) errors gets a
	// generic 500 — those are internal failures, not client bad requests.
	httpx.SetErrorHandlerCtx(func(_ context.Context, err error) (int, any) {
		code, body := errors.GrpcErrorResponse(err)
		return code, body
	})

	// No CORS: the admin panel is same-origin behind the admin-frontend BFF
	// proxy, so no browser ever calls adminway cross-origin.
	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	ctx := svc.NewServiceContext(c)

	// Rate limit first so abusive requests to the unauthenticated auth routes
	// are rejected before any handler or auth overhead.
	server.Use(ctx.RateLimit)

	handler.RegisterHandlers(server, ctx)

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
