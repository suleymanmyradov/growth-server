package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/consumer/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/consumer/internal/svc"
	"github.com/zeromicro/go-zero/core/conf"
)

var configFile = flag.String("f", "etc/ai-coach.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
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
