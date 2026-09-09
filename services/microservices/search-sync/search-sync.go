package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/configsafe"
	"github.com/suleymanmyradov/growth-server/services/microservices/search-sync/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/search-sync/internal/indexer"
	"github.com/suleymanmyradov/growth-server/services/microservices/search-sync/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/search-sync/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/search-sync/internal/syncer"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
)

var configFile = flag.String("f", "etc/search-sync.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c, conf.UseEnv())
	c.MustSetUp()
	logx.Infof("starting search-sync with config: %+v", configsafe.MaskSecrets(c))

	// Set defaults
	if c.Sync.ReconcileInterval == 0 {
		c.Sync.ReconcileInterval = 2 * time.Minute
	}
	if c.Sync.FullReconcileInterval == 0 {
		c.Sync.FullReconcileInterval = 24 * time.Hour
	}
	if c.Meili.Index == "" {
		c.Meili.Index = "growth_search"
	}

	ctx := svc.NewServiceContext(c)
	defer ctx.Close()

	repo := repository.NewRepository(ctx.Pool)
	idx := indexer.NewMeiliIndexer(ctx.Index, ctx.MemoryIndex)
	sync := syncer.NewSyncer(repo, idx, c)

	if c.Backfill {
		logx.Info("running full reconcile (backfill mode)...")
		sync.ReconcileFull(context.Background())
		logx.Info("backfill complete")
		return
	}

	rootCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go sync.Run(rootCtx)

	logx.Info("search-sync started")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logx.Info("shutting down search-sync...")
	cancel()
	// Give workers a moment to finish current batch
	time.Sleep(500 * time.Millisecond)
}
