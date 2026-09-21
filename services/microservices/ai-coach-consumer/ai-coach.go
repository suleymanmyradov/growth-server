package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/suleymanmyradov/growth-server/pkg/sentryx"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach-consumer/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach-consumer/internal/svc"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/prometheus"
	"github.com/zeromicro/go-zero/core/trace"
)

var configFile = flag.String("f", "etc/ai-coach.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c, conf.UseEnv())
	sentryx.Init("ai-coach-consumer")
	defer sentryx.Flush()
	if len(c.Log.ServiceName) == 0 {
		c.Log.ServiceName = "ai-coach-consumer"
	}
	logx.MustSetup(c.Log)
	prometheus.StartAgent(c.Prometheus)
	trace.StartAgent(c.Telemetry)
	ctx := svc.NewServiceContext(c)

	// Start Kafka consumer.
	ctx.StartConsumers()

	// Wait for termination signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	// Graceful shutdown.
	ctx.Close()
}
